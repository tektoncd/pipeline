package github

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jenkins-x/go-scm/scm"
)

const (
	// https://developer.github.com/changes/2016-09-14-Integrations-Early-Access/
	mediaTypeIntegrationPreview = "application/vnd.github.machine-man-preview+json"
)

type appService struct {
	client *wrapper
}

type installationToken struct {
	Token     *string    `json:"token,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func (s *appService) CreateInstallationToken(ctx context.Context, id int64) (*scm.InstallationToken, *scm.Response, error) {
	path := fmt.Sprintf("app/installations/%v/access_tokens", id)
	out := new(installationToken)

	req := &scm.Request{
		Method: http.MethodPost,
		Path:   path,
		Header: map[string][]string{
			// TODO: remove custom Accept header when this API fully launches.
			"Accept": {mediaTypeIntegrationPreview},
		},
	}

	res, err := s.client.doRequest(ctx, req, nil, out)
	return convertInstallationToken(out), res, err
}

func (s *appService) GetRepositoryInstallation(ctx context.Context, fullName string) (*scm.Installation, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/installation", fullName)
	out := new(installation)

	req := &scm.Request{
		Method: http.MethodGet,
		Path:   path,
		Header: map[string][]string{
			// TODO: remove custom Accept header when this API fully launches.
			"Accept": {mediaTypeIntegrationPreview},
		},
	}

	res, err := s.client.doRequest(ctx, req, nil, out)
	return convertInstallation(out), res, err
}

func (s *appService) GetOrganisationInstallation(ctx context.Context, organisation string) (*scm.Installation, *scm.Response, error) {
	path := fmt.Sprintf("orgs/%s/installation", organisation)
	out := new(installation)

	req := &scm.Request{
		Method: http.MethodGet,
		Path:   path,
		Header: map[string][]string{
			// TODO: remove custom Accept header when this API fully launches.
			"Accept": {mediaTypeIntegrationPreview},
		},
	}

	res, err := s.client.doRequest(ctx, req, nil, out)
	return convertInstallation(out), res, err
}

func (s *appService) GetUserInstallation(ctx context.Context, user string) (*scm.Installation, *scm.Response, error) {
	path := fmt.Sprintf("users/%s/installation", user)
	out := new(installation)

	req := &scm.Request{
		Method: http.MethodGet,
		Path:   path,
		Header: map[string][]string{
			// TODO: remove custom Accept header when this API fully launches.
			"Accept": {mediaTypeIntegrationPreview},
		},
	}

	res, err := s.client.doRequest(ctx, req, nil, out)
	return convertInstallation(out), res, err
}

func convertInstallationToken(src *installationToken) *scm.InstallationToken {
	dst := &scm.InstallationToken{
		ExpiresAt: src.ExpiresAt,
	}
	if src.Token != nil {
		dst.Token = *src.Token
	}
	return dst
}
