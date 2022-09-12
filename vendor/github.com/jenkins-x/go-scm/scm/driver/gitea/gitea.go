// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gitea implements a Gitea client.
package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/sdk/gitea"
	"github.com/jenkins-x/go-scm/scm"
)

// NewWebHookService creates a new instance of the webhook service without the rest of the client
func NewWebHookService() scm.WebhookService {
	return &webhookService{nil}
}

// New returns a new Gitea API client without a token set
func New(uri string) (*scm.Client, error) {
	return NewWithToken(uri, "")
}

// NewWithToken returns a new Gitea API client with the token set.
func NewWithToken(uri, token string) (*scm.Client, error) {
	base, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	client := &wrapper{Client: new(scm.Client)}
	client.GiteaClient, err = gitea.NewClient(base.String(), gitea.SetToken(token))

	if err != nil {
		return nil, err
	}
	client.BaseURL = base
	// initialize services
	client.Driver = scm.DriverGitea
	client.Contents = &contentService{client}
	client.Git = &gitService{client}
	client.Issues = &issueService{client}
	client.Milestones = &milestoneService{client}
	client.Organizations = &organizationService{client}
	client.PullRequests = &pullService{&issueService{client}}
	client.Repositories = &repositoryService{client}
	client.Reviews = &reviewService{client}
	client.Releases = &releaseService{client}
	client.Users = &userService{client}
	client.Webhooks = &webhookService{client}
	return client.Client, nil
}

// NewWithBasicAuth returns a new Gitea API client with the basic auth set.
func NewWithBasicAuth(uri, user, password string) (*scm.Client, error) {
	base, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	client := &wrapper{Client: new(scm.Client)}
	client.GiteaClient, err = gitea.NewClient(base.String(), gitea.SetBasicAuth(user, password))

	if err != nil {
		return nil, err
	}
	client.BaseURL = base
	// initialize services
	client.Driver = scm.DriverGitea
	client.Contents = &contentService{client}
	client.Git = &gitService{client}
	client.Issues = &issueService{client}
	client.Milestones = &milestoneService{client}
	client.Organizations = &organizationService{client}
	client.PullRequests = &pullService{&issueService{client}}
	client.Repositories = &repositoryService{client}
	client.Reviews = &reviewService{client}
	client.Users = &userService{client}
	client.Webhooks = &webhookService{client}
	return client.Client, nil
}

// wraper wraps the Client to provide high level helper functions
// for making http requests and unmarshaling the response.
type wrapper struct {
	*scm.Client
	GiteaClient *gitea.Client
}

// do wraps the Client.Do function by creating the Request and
// unmarshalling the response.
func (c *wrapper) do(ctx context.Context, method, path string, in, out interface{}) (*scm.Response, error) {
	req := &scm.Request{
		Method: method,
		Path:   path,
	}
	// if we are posting or putting data, we need to
	// write it to the body of the request.
	if in != nil {
		buf := new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(in) // #nosec
		if err != nil {
			return nil, err
		}
		req.Header = map[string][]string{
			"Content-Type": {"application/json"},
		}
		req.Body = buf
	}

	// execute the http request
	res, err := c.Client.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// if an error is encountered, unmarshal and return the
	// error response.
	if res.Status > 300 {
		return res, errors.New(
			http.StatusText(res.Status),
		)
	}

	if out == nil {
		return res, nil
	}

	// if raw output is expected, copy to the provided
	// buffer and exit.
	if w, ok := out.(io.Writer); ok {
		_, err := io.Copy(w, res.Body)
		if err != nil {
			return res, err
		}
		return res, nil
	}

	// if a json response is expected, parse and return
	// the json response.
	return res, json.NewDecoder(res.Body).Decode(out)
}

// toSCMResponse creates a new Response for the provided
// http.Response. r must not be nil.
func toSCMResponse(r *gitea.Response) *scm.Response {
	if r == nil {
		return nil
	}
	res := &scm.Response{
		Status: r.StatusCode,
		Header: r.Header,
		Body:   r.Body,
	}
	res.PopulatePageValues()
	return res
}

func toGiteaListOptions(in *scm.ListOptions) gitea.ListOptions {
	return gitea.ListOptions{
		Page:     in.Page,
		PageSize: in.Size,
	}
}
