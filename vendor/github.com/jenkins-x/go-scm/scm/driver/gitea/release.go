package gitea

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"code.gitea.io/sdk/gitea"
	"github.com/jenkins-x/go-scm/scm"
)

type releaseService struct {
	client *wrapper
}

func (s *releaseService) Find(ctx context.Context, repo string, id int) (*scm.Release, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	out, resp, err := s.client.GiteaClient.GetRelease(namespace, name, int64(id))
	return convertRelease(out), toSCMResponse(resp), err
}

func (s *releaseService) FindByTag(ctx context.Context, repo, tag string) (*scm.Release, *scm.Response, error) {

	namespace, name := scm.Split(repo)

	// newer versions of gitea have a GetReleaseByTag that doesn't 500 error
	if err := s.client.GiteaClient.CheckServerVersionConstraint(">= 1.13.2"); err == nil {
		out, resp, err := s.client.GiteaClient.GetReleaseByTag(namespace, name, tag)
		if err == nil {
			// There is a bug where a release that is a tag is returned - filter these out based on contents of the fields
			// https://github.com/go-gitea/gitea/pull/14397
			if out != nil && out.Title == "" && out.Note == "" {
				return nil, nil, scm.ErrNotFound
			}
		}
		return convertRelease(out), toSCMResponse(resp), err
	}

	// older gitea version a broken `GetReleaseByTag`, so use `ListReleases` and iterate over each page
	// https://github.com/go-gitea/gitea/commit/5ee09d3c8161280c72be8b02cd7ea354c1f55331
	var releases []*gitea.Release
	var err error
	opts := scm.ReleaseListOptions{
		Page: 1,
		Size: 100,
	}

	scanPages := 1000
	for opts.Page <= scanPages {
		releases, _, err = s.client.GiteaClient.ListReleases(namespace, name, gitea.ListReleasesOptions{ListOptions: releaseListOptionsToGiteaListOptions(opts)})
		if err != nil {
			return nil, nil, err
		}
		if len(releases) == 0 {
			// no more pages to scan, release was not found
			return nil, nil, scm.ErrNotFound
		}
		for _, r := range releases {
			if strings.EqualFold(strings.ToLower(r.TagName), strings.ToLower(tag)) {
				return convertRelease(r), nil, nil
			}
		}
		opts.Page++
	}
	return nil, nil, fmt.Errorf("gave up scanning for release after %v pages, upgrade gitea to >= 1.13.2", scanPages)
}

func (s *releaseService) List(ctx context.Context, repo string, opts scm.ReleaseListOptions) ([]*scm.Release, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	out, resp, err := s.client.GiteaClient.ListReleases(namespace, name, gitea.ListReleasesOptions{ListOptions: releaseListOptionsToGiteaListOptions(opts)})
	return convertReleaseList(out), toSCMResponse(resp), err
}

func (s *releaseService) Create(ctx context.Context, repo string, input *scm.ReleaseInput) (*scm.Release, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	out, resp, err := s.client.GiteaClient.CreateRelease(namespace, name, gitea.CreateReleaseOption{
		TagName:      input.Tag,
		Target:       input.Commitish,
		Title:        input.Title,
		Note:         input.Description,
		IsDraft:      input.Draft,
		IsPrerelease: input.Prerelease,
	})
	return convertRelease(out), toSCMResponse(resp), err
}

func (s *releaseService) Delete(ctx context.Context, repo string, id int) (*scm.Response, error) {
	namespace, name := scm.Split(repo)
	resp, err := s.client.GiteaClient.DeleteRelease(namespace, name, int64(id))
	return toSCMResponse(resp), err
}

func (s *releaseService) DeleteByTag(ctx context.Context, repo, tag string) (*scm.Response, error) {
	rel, _, err := s.FindByTag(ctx, repo, tag)
	if err != nil {
		return nil, err
	}
	return s.Delete(ctx, repo, rel.ID)
}

func (s *releaseService) Update(ctx context.Context, repo string, id int, input *scm.ReleaseInput) (*scm.Release, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	out, resp, err := s.client.GiteaClient.EditRelease(namespace, name, int64(id), gitea.EditReleaseOption{
		TagName:      input.Tag,
		Target:       input.Commitish,
		Title:        input.Title,
		Note:         input.Description,
		IsDraft:      &input.Draft,
		IsPrerelease: &input.Prerelease,
	})
	return convertRelease(out), toSCMResponse(resp), err
}

func (s *releaseService) UpdateByTag(ctx context.Context, repo, tag string, input *scm.ReleaseInput) (*scm.Release, *scm.Response, error) {
	rel, _, err := s.FindByTag(ctx, repo, tag)
	if err != nil {
		return nil, nil, err
	}
	return s.Update(ctx, repo, rel.ID, input)
}

func convertReleaseList(from []*gitea.Release) []*scm.Release {
	var to []*scm.Release
	for _, m := range from {
		to = append(to, convertRelease(m))
	}
	return to
}

// ConvertAPIURLToHTMLURL converts an release API endpoint into a html endpoint
func ConvertAPIURLToHTMLURL(apiURL, tagName string) string {
	// "url": "https://try.gitea.com/api/v1/repos/octocat/Hello-World/123",
	// "html_url": "https://try.gitea.com/octocat/Hello-World/releases/tag/v1.0.0",
	// the url field is the API url, not the html url, so until go-sdk v0.13.3, build it ourselves
	link, err := url.Parse(apiURL)
	if err != nil {
		return ""
	}

	pathParts := strings.Split(link.Path, "/")
	if len(pathParts) != 7 {
		return ""
	}
	link.Path = fmt.Sprintf("/%s/%s/releases/tag/%s", pathParts[4], pathParts[5], tagName)
	return link.String()

}

func convertRelease(from *gitea.Release) *scm.Release {
	return &scm.Release{
		ID:          int(from.ID),
		Title:       from.Title,
		Description: from.Note,
		// Link:        from.HTMLURL,
		Link:       ConvertAPIURLToHTMLURL(from.URL, from.TagName),
		Tag:        from.TagName,
		Commitish:  from.Target,
		Draft:      from.IsDraft,
		Prerelease: from.IsPrerelease,
		Created:    from.CreatedAt,
		Published:  from.PublishedAt,
	}
}

func releaseListOptionsToGiteaListOptions(in scm.ReleaseListOptions) gitea.ListOptions {
	return gitea.ListOptions{
		Page:     in.Page,
		PageSize: in.Size,
	}
}
