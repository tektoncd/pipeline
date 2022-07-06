package gitlab

import (
	"context"
	"fmt"

	"github.com/jenkins-x/go-scm/scm"
)

type releaseService struct {
	client *wrapper
}

type release struct {
	Title       string `json:"name"`
	Description string `json:"description"`
	Tag         string `json:"tag_name"`
	Commit      struct {
		ID string `json:"id"`
	} `json:"commit"`
}

type releaseInput struct {
	Title       string `json:"name"`
	Description string `json:"description"`
	Tag         string `json:"tag_name"`
}

func (s *releaseService) Find(ctx context.Context, repo string, id int) (*scm.Release, *scm.Response, error) {
	// this could be implemented by List and filter but would be to expensive
	panic("gitlab only allows to find a release by tag")
}

func (s *releaseService) FindByTag(ctx context.Context, repo, tag string) (*scm.Release, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/releases/%s", encode(repo), tag)
	out := new(release)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertRelease(out), res, err
}

func (s *releaseService) List(ctx context.Context, repo string, opts scm.ReleaseListOptions) ([]*scm.Release, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/releases", encode(repo))
	out := []*release{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertReleaseList(out), res, err
}

func (s *releaseService) Create(ctx context.Context, repo string, input *scm.ReleaseInput) (*scm.Release, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/releases", encode(repo))
	in := &releaseInput{
		Title:       input.Title,
		Description: input.Description,
		Tag:         input.Tag,
	}
	out := new(release)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertRelease(out), res, err
}

func (s *releaseService) Delete(ctx context.Context, repo string, id int) (*scm.Response, error) {
	// this could be implemented by List and filter but would be to expensive
	panic("gitlab only allows to delete a release by tag")
}

func (s *releaseService) DeleteByTag(ctx context.Context, repo, tag string) (*scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/releases/%s", encode(repo), tag)
	return s.client.do(ctx, "DELETE", path, nil, nil)
}

func (s *releaseService) Update(ctx context.Context, repo string, id int, input *scm.ReleaseInput) (*scm.Release, *scm.Response, error) {
	// this could be implemented by List and filter but would be to expensive
	panic("gitlab only allows to update a release by tag")
}

func (s *releaseService) UpdateByTag(ctx context.Context, repo, tag string, input *scm.ReleaseInput) (*scm.Release, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/releases/%s", encode(repo), tag)
	in := &releaseInput{}
	if input.Title != "" {
		in.Title = input.Title
	}
	if input.Description != "" {
		in.Description = input.Description
	}
	if input.Tag != "" {
		in.Tag = input.Tag
	}
	out := new(release)
	res, err := s.client.do(ctx, "PUT", path, in, out)
	return convertRelease(out), res, err
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
		ID:          0,
		Title:       from.Title,
		Description: from.Description,
		Link:        "",
		Tag:         from.Tag,
		Commitish:   from.Commit.ID,
		Draft:       false, // not supported by gitlab
		Prerelease:  false, // not supported by gitlab
	}
}
