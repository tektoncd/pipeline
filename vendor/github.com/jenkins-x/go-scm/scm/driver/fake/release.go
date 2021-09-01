package fake

import (
	"context"
	"strconv"
	"time"

	"github.com/jenkins-x/go-scm/scm"
)

type releaseService struct {
	client *wrapper
	data   *Data
}

func (r *releaseService) Find(_ context.Context, repo string, number int) (*scm.Release, *scm.Response, error) {
	release := r.releaseMap(repo)[number]
	if release == nil {
		return nil, nil, scm.ErrNotFound
	}
	return release, nil, nil
}

func (r *releaseService) releaseMap(repo string) map[int]*scm.Release {
	if r.data.Releases == nil {
		r.data.Releases = map[string]map[int]*scm.Release{}
	}
	releaseMap := r.data.Releases[repo]
	if releaseMap == nil {
		releaseMap = map[int]*scm.Release{}
		r.data.Releases[repo] = releaseMap
	}
	return releaseMap
}

func (r *releaseService) FindByTag(_ context.Context, repo string, tag string) (*scm.Release, *scm.Response, error) {
	for _, rel := range r.releaseMap(repo) {
		if rel.Tag == tag {
			return rel, nil, nil
		}
	}
	return nil, nil, scm.ErrNotFound
}

func (r *releaseService) List(_ context.Context, repo string, options scm.ReleaseListOptions) ([]*scm.Release, *scm.Response, error) {
	var answer []*scm.Release
	for _, rel := range r.releaseMap(repo) {
		answer = append(answer, rel)
	}
	if len(answer) == 0 {
		return nil, nil, scm.ErrNotFound
	}
	return answer, nil, nil
}

func (r *releaseService) Create(_ context.Context, repo string, input *scm.ReleaseInput) (*scm.Release, *scm.Response, error) {
	m := r.releaseMap(repo)
	id := len(m)
	now := time.Now()
	release := &scm.Release{
		ID:          id,
		Title:       input.Title,
		Description: input.Description,
		Link:        "https://fake.git/" + repo + "/releases/release/" + strconv.Itoa(id),
		Tag:         input.Tag,
		Commitish:   input.Commitish,
		Draft:       input.Draft,
		Prerelease:  input.Prerelease,
		Created:     now,
		Published:   now,
	}
	m[id] = release
	return release, nil, nil
}

func (r *releaseService) Update(_ context.Context, repo string, number int, input *scm.ReleaseInput) (*scm.Release, *scm.Response, error) {
	rel := r.releaseMap(repo)[number]
	if rel == nil {
		return nil, nil, scm.ErrNotFound
	}
	if input.Commitish != "" {
		rel.Commitish = input.Commitish
	}
	if input.Description != "" {
		rel.Description = input.Description
	}
	if input.Tag != "" {
		rel.Tag = input.Tag
	}
	if input.Title != "" {
		rel.Title = input.Title
	}
	rel.Draft = input.Draft
	rel.Prerelease = input.Prerelease
	return nil, nil, nil
}

func (r *releaseService) UpdateByTag(ctx context.Context, repo string, tag string, input *scm.ReleaseInput) (*scm.Release, *scm.Response, error) {
	rel, _, _ := r.FindByTag(ctx, repo, tag)
	return r.Update(ctx, repo, rel.ID, input)
}

func (r *releaseService) Delete(_ context.Context, repo string, number int) (*scm.Response, error) {
	m := r.releaseMap(repo)
	delete(m, number)
	return nil, nil
}

func (r *releaseService) DeleteByTag(ctx context.Context, repo string, tag string) (*scm.Response, error) {
	rel, _, _ := r.FindByTag(ctx, repo, tag)
	return r.Delete(ctx, repo, rel.ID)
}
