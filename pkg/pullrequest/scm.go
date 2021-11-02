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
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/oauth2"

	"crypto/tls"

	"github.com/jenkins-x/go-scm/scm/driver/github"
	"github.com/jenkins-x/go-scm/scm/driver/gitlab"
	"go.uber.org/zap"
)

// NewSCMHandler returns a new Handler  for the given URL, provider and token
func NewSCMHandler(logger *zap.SugaredLogger, raw, provider, token string, skipTLSVerify bool) (*Handler, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	if provider == "" {
		p, err := guessProvider(u)
		if err != nil {
			return nil, err
		}
		provider = p
	}
	logger = logger.With(zap.String("provider", provider))

	var handler *Handler
	switch provider {
	case "github":
		handler, err = githubHandlerFromURL(u, token, skipTLSVerify, logger)
	case "gitlab":
		handler, err = gitlabHandlerFromURL(u, token, skipTLSVerify, logger)
	default:
		return nil, fmt.Errorf("unsupported pr url: %s", raw)
	}
	return handler, err
}

func githubHandlerFromURL(u *url.URL, token string, skipTLSVerify bool, logger *zap.SugaredLogger) (*Handler, error) {
	split := strings.Split(u.Path, "/")
	if len(split) < 5 {
		return nil, fmt.Errorf("could not determine PR from URL: %v", u)
	}
	owner, repo, pr := split[1], split[2], split[4]
	prNumber, err := strconv.Atoi(pr)
	if err != nil {
		return nil, fmt.Errorf("error parsing PR number: %s", pr)
	}
	logger = logger.With(
		zap.String("owner", owner),
		zap.String("repo", repo),
		zap.String("pr", pr),
	)

	// GitHub uses a different URL format than GHE.
	// GHE is http(s)://[hostname]/api/v3
	// (https://developer.github.com/enterprise/2.17/v3/#schema),
	// GitHub uses https://api.github.com (http will be redirected to https).
	//
	// Here we allow the scheme to be set from the incoming URL instead of
	// always defaulting to HTTPs to allow for easier proxy interception of
	// requests in tests.
	var prefix string
	if u.Host == "github.com" {
		prefix = fmt.Sprintf("%s://api.github.com", u.Scheme)
	} else {
		prefix = fmt.Sprintf("%s://%s/api/v3", u.Scheme, u.Host)
	}
	client, err := github.New(prefix)
	if err != nil {
		return nil, fmt.Errorf("error creating client: %w", err)
	}
	ownerRepo := fmt.Sprintf("%s/%s", owner, repo)

	// Make sure to keep the default transport. This has builtin features like
	// recognizing proxy settings that are useful to us.
	t := http.DefaultTransport.(*http.Transport).Clone()
	// gosec complains that we're setting the InsecureSkipVerify option to bypass
	// security checks. As long as this is generally set to false (which is the
	// case by default), this should be fine.
	// #nosec G402
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: skipTLSVerify}

	if token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		client.Client = &http.Client{
			Transport: &oauth2.Transport{
				Source: ts,
				Base:   t,
			},
		}
	} else {
		client.Client = &http.Client{
			Transport: t,
		}
	}

	h := NewHandler(logger, client, ownerRepo, prNumber)
	return h, nil
}

func gitlabHandlerFromURL(u *url.URL, token string, skipTLSVerify bool, logger *zap.SugaredLogger) (*Handler, error) {
	// The project name can be multiple /'s deep, so split on / and work from right to left.
	split := strings.Split(u.Path, "/")

	// The PR number should be the last element.
	last := len(split) - 1
	prNum := split[last]
	prInt, err := strconv.Atoi(prNum)
	if err != nil {
		return nil, fmt.Errorf("unable to parse pr as number from %s", u)
	}

	// Next we sanity check that this is a correct url. The next to last element should be "merge_requests"
	if split[last-1] != "merge_requests" {
		return nil, fmt.Errorf("invalid gitlab url: %s", u)
	}

	// Next, we rejoin everything else into the project field.
	project := strings.Join(split[1:last-1], "/")
	logger = logger.With(
		zap.String("project", project),
		zap.String("pr", prNum),
	)
	client := gitlab.NewDefault()
	if u.Host != "gitlab.com" {
		var err error
		client, err = gitlab.New(fmt.Sprintf("%s://%s", u.Scheme, u.Host))
		if err != nil {
			return nil, fmt.Errorf("error creating client: %w", err)
		}
	}

	t := http.DefaultTransport.(*http.Transport).Clone()
	// #nosec G402
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: skipTLSVerify}

	if token != "" {
		client.Client = &http.Client{
			Transport: &gitlabClient{
				token:     token,
				transport: t,
			},
		}
	} else {
		client.Client = &http.Client{
			Transport: t,
		}
	}

	return NewHandler(logger, client, project, prInt), nil
}

// gitlab client wraps a normal http client, adding support for private-token auth.
type gitlabClient struct {
	token     string
	transport http.RoundTripper
}

// RoundTrip handles authentication for the gitlabClient
func (g *gitlabClient) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Add("Private-Token", g.token)
	return g.transport.RoundTrip(r)
}

func guessProvider(u *url.URL) (string, error) {
	switch {
	case strings.Contains(u.Hostname(), "github"):
		return "github", nil
	case strings.Contains(u.Hostname(), "gitlab"):
		return "gitlab", nil
	}
	return "", fmt.Errorf("unable to guess scm provider from url: %s", u)
}
