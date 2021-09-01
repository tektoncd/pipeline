// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scm

import (
	"context"
	"errors"

	"io"
	"net/http"

	"net/url"
	"strconv"
	"strings"
	"sync"
)

var (
	// ErrNotFound indicates a resource is not found.
	ErrNotFound = errors.New("Not Found")

	// ErrNotSupported indicates a resource endpoint is not
	// supported or implemented.
	ErrNotSupported = errors.New("Not Supported")

	// ErrNotAuthorized indicates the request is not
	// authorized or the user does not have access to the
	// resource.
	ErrNotAuthorized = errors.New("Not Authorized")

	// ErrForbidden indicates the user does not have access to
	// the resource, this is similar to 401, but in this case,
	// re-authenticating will make no difference.
	ErrForbidden = errors.New("Forbidden")
)

type (
	// Request represents an HTTP request.
	Request struct {
		Method string
		Path   string
		Header http.Header
		Body   io.Reader
	}

	// Response represents an HTTP response.
	Response struct {
		ID     string
		Status int
		Header http.Header
		Body   io.ReadCloser

		Page Page // Page values
		Rate Rate // Rate limit snapshot
	}

	// Page represents parsed link rel values for
	// pagination.
	Page struct {
		Next    int
		NextURL string
		Last    int
		First   int
		Prev    int
	}

	// Rate represents the rate limit for the current
	// client.
	Rate struct {
		Limit     int
		Remaining int
		Reset     int64
	}

	// ListOptions specifies optional pagination
	// parameters.
	ListOptions struct {
		URL  string
		Page int
		Size int
		From string
		To   string
	}

	// GraphQLService the API to performing GraphQL queries
	GraphQLService interface {
		Query(ctx context.Context, q interface{}, vars map[string]interface{}) error
	}

	// Client manages communication with a version control
	// system API.
	Client struct {
		mu sync.Mutex

		// HTTP client used to communicate with the API.
		Client *http.Client

		// Base URL for API requests.
		BaseURL    *url.URL
		GraphQLURL *url.URL

		// Username is the optional user name for the client
		Username string

		// Services used for communicating with the API.
		Driver        Driver
		Apps          AppService
		Contents      ContentService
		Deployments   DeploymentService
		Git           GitService
		GraphQL       GraphQLService
		Organizations OrganizationService
		Issues        IssueService
		Milestones    MilestoneService
		Releases      ReleaseService
		PullRequests  PullRequestService
		Repositories  RepositoryService
		Reviews       ReviewService
		Users         UserService
		Webhooks      WebhookService
		Commits       CommitService

		// DumpResponse optionally specifies a function to
		// dump the the response body for debugging purposes.
		// This can be set to httputil.DumpResponse.
		DumpResponse func(*http.Response, bool) ([]byte, error)

		// snapshot of the request rate limit.
		rate Rate
	}
)

// Rate returns a snapshot of the request rate limit for
// the current client.
func (c *Client) Rate() Rate {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rate
}

// SetRate set the last recorded request rate limit for
// the current client.
func (c *Client) SetRate(rate Rate) {
	c.mu.Lock()
	c.rate = rate
	c.mu.Unlock()
}

// Do sends an API request and returns the API response.
// The API response is JSON decoded and stored in the
// value pointed to by v, or returned as an error if an
// API error has occurred. If v implements the io.Writer
// interface, the raw response will be written to v,
// without attempting to decode it.
func (c *Client) Do(ctx context.Context, in *Request) (*Response, error) {
	uri, err := c.BaseURL.Parse(in.Path)
	if err != nil {
		return nil, err
	}

	// creates a new http request with context.
	req, err := http.NewRequest(in.Method, uri.String(), in.Body)
	if err != nil {
		return nil, err
	}
	// hack to prevent the client from un-escaping the
	// encoded github path parameters when parsing the url.
	if strings.Contains(in.Path, "%2F") {
		req.URL.Opaque = strings.Split(req.URL.RawPath, "?")[0]
	}

	req = req.WithContext(ctx)
	if in.Header != nil {
		req.Header = in.Header
	}

	// use the default client if none provided.
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// dumps the response for debugging purposes.
	if c.DumpResponse != nil {
		_, err = c.DumpResponse(res, true)
	}
	return newResponse(res), err
}

// newResponse creates a new Response for the provided
// http.Response. r must not be nil.
func newResponse(r *http.Response) *Response {
	res := &Response{
		Status: r.StatusCode,
		Header: r.Header,
		Body:   r.Body,
	}
	res.PopulatePageValues()
	return res
}

// PopulatePageValues parses the HTTP Link response headers
// and populates the various pagination link values in the
// Response.
//
// Copyright 2013 The go-github AUTHORS. All rights reserved.
// https://github.com/google/go-github
func (r *Response) PopulatePageValues() {
	links := strings.Split(r.Header.Get("Link"), ",")
	for _, link := range links {
		segments := strings.Split(strings.TrimSpace(link), ";")

		if len(segments) < 2 {
			continue
		}

		if !strings.HasPrefix(segments[0], "<") ||
			!strings.HasSuffix(segments[0], ">") {
			continue
		}

		url, err := url.Parse(segments[0][1 : len(segments[0])-1])
		if err != nil {
			continue
		}

		page := url.Query().Get("page")
		if page == "" {
			continue
		}

		for _, segment := range segments[1:] {
			switch strings.TrimSpace(segment) {
			case `rel="next"`:
				r.Page.Next, _ = strconv.Atoi(page)
			case `rel="prev"`:
				r.Page.Prev, _ = strconv.Atoi(page)
			case `rel="first"`:
				r.Page.First, _ = strconv.Atoi(page)
			case `rel="last"`:
				r.Page.Last, _ = strconv.Atoi(page)
			}
		}
	}
}
