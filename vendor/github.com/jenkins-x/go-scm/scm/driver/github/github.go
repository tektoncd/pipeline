// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package github implements a GitHub client.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jenkins-x/go-scm/scm"
	githubql "github.com/shurcooL/githubv4"
)

// Abort requests that don't return in 5 mins. Longest graphql calls can
// take up to 2 minutes. This limit should ensure all successful calls return
// but will prevent an indefinite stall if GitHub never responds.
const maxRequestTime = 5 * time.Minute

// NewWebHookService creates a new instance of the webhook service without the rest of the client
func NewWebHookService() scm.WebhookService {
	return &webhookService{nil}
}

// New returns a new GitHub API client.
func New(uri string) (*scm.Client, error) {
	base, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	client := &wrapper{new(scm.Client)}
	client.BaseURL = base
	// initialize services
	client.Driver = scm.DriverGithub
	client.Contents = &contentService{client}
	client.Deployments = &deploymentService{client}
	client.Git = &gitService{client}
	client.Issues = &issueService{client}
	client.Milestones = &milestoneService{client}
	client.Releases = &releaseService{client}
	client.Organizations = &organizationService{client}
	client.PullRequests = &pullService{&issueService{client}}
	client.Repositories = &repositoryService{client}
	client.Reviews = &reviewService{client}
	client.Users = &userService{client}
	client.Webhooks = &webhookService{client}
	client.Apps = &appService{client}

	graphqlEndpoint := scm.URLJoin(uri, "/graphql")
	if strings.HasSuffix(uri, "/api/v3") {
		graphqlEndpoint = scm.URLJoin(uri[0:len(uri)-2], "graphql")
	}
	client.GraphQLURL, err = url.Parse(graphqlEndpoint)
	if err != nil {
		return nil, err
	}
	client.GraphQL = &dynamicGraphQLClient{client, graphqlEndpoint}

	return client.Client, nil
}

type dynamicGraphQLClient struct {
	wrapper         *wrapper
	graphqlEndpoint string
}

func (d *dynamicGraphQLClient) Query(ctx context.Context, q interface{}, vars map[string]interface{}) error {
	httpClient := d.wrapper.Client.Client
	if httpClient != nil {

		transport := httpClient.Transport
		if transport != nil {
			query := githubql.NewEnterpriseClient(
				d.graphqlEndpoint,
				&http.Client{
					Timeout:   maxRequestTime,
					Transport: transport,
				})
			return query.Query(ctx, q, vars)
		}
	}
	fmt.Println("WARNING: no http transport configured for GraphQL and GitHub")
	return nil
}

// NewDefault returns a new GitHub API client using the
// default api.github.com address.
func NewDefault() *scm.Client {
	client, _ := New("https://api.github.com")
	return client
}

// wraper wraps the Client to provide high level helper functions
// for making http requests and unmarshaling the response.
type wrapper struct {
	*scm.Client
}

// do wraps the Client.Do function by creating the Request and
// unmarshalling the response.
func (c *wrapper) do(ctx context.Context, method, path string, in, out interface{}) (*scm.Response, error) {
	req := &scm.Request{
		Method: method,
		Path:   path,
	}
	return c.doRequest(ctx, req, in, out)
}

func (c *wrapper) doRequest(ctx context.Context, req *scm.Request, in, out interface{}) (*scm.Response, error) {
	// if we are posting or putting data, we need to
	// write it to the body of the request.
	if in != nil {
		buf := new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(in) // #nosec
		if err != nil {
			return nil, err
		}
		if req.Header == nil {
			req.Header = map[string][]string{}
		}
		req.Header["Content-Type"] = []string{"application/json"}
		req.Body = buf
	}

	// execute the http request
	res, err := c.Client.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// parse the github request id.
	res.ID = res.Header.Get("X-GitHub-Request-Id")

	// parse the github rate limit details.
	res.Rate.Limit, _ = strconv.Atoi(
		res.Header.Get("X-RateLimit-Limit"),
	)
	res.Rate.Remaining, _ = strconv.Atoi(
		res.Header.Get("X-RateLimit-Remaining"),
	)
	res.Rate.Reset, _ = strconv.ParseInt(
		res.Header.Get("X-RateLimit-Reset"), 10, 64,
	)

	// snapshot the request rate limit
	c.Client.SetRate(res.Rate)

	// if an error is encountered, unmarshal and return the
	// error response.
	if res.Status > 300 {
		if res.Status == 404 {
			return res, scm.ErrNotFound
		}
		return res, errors.New(
			http.StatusText(res.Status),
		)
	}

	if out == nil {
		return res, nil
	}

	// if a json response is expected, parse and return
	// the json response.
	return res, json.NewDecoder(res.Body).Decode(out)
}

// Error represents a Github error.
type Error struct {
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return e.Message
}
