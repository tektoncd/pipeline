// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package azure implements a azure client.
package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/jenkins-x/go-scm/scm"
)

// New returns a new azure API client.
func New(uri string) (*scm.Client, error) {
	base, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	client := &wrapper{
		new(scm.Client),
	}
	client.BaseURL = base
	// initialize services
	client.Driver = scm.DriverAzure
	client.Contents = &contentService{client}
	client.Git = &gitService{client}
	client.Issues = &issueService{client}
	client.Organizations = &organizationService{client}
	client.PullRequests = &pullService{&issueService{client}}
	client.Repositories = &RepositoryService{client}
	client.Reviews = &reviewService{client}
	client.Users = &userService{client}
	client.Webhooks = &webhookService{client}
	return client.Client, nil
}

// NewDefault returns a new azure API client.
func NewDefault() *scm.Client {
	client, _ := New("https://dev.azure.com")
	return client
}

// wrapper wraps the Client to provide high level helper functions for making http requests and unmarshaling the response.
type wrapper struct {
	*scm.Client
}

// do wraps the Client.Do function by creating the Request and unmarshalling the response.
func (c *wrapper) do(ctx context.Context, method, path string, in, out interface{}) (*scm.Response, error) {
	req := &scm.Request{
		Method: method,
		Path:   path,
	}
	// if we are posting or putting data, we need to write it to the body of the request.
	if in != nil {
		buf := new(bytes.Buffer)
		_ = json.NewEncoder(buf).Encode(in)
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

	// error response.
	if res.Status > 300 {
		err := new(Error)
		_ = json.NewDecoder(res.Body).Decode(err)
		return res, err
	}
	// the following is used for debugging purposes.
	// bytes, err := io.ReadAll(res.Body)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Println(string(bytes))

	if out == nil {
		return res, nil
	}
	// if a json response is expected, parse and return the json response.
	decodeErr := json.NewDecoder(res.Body).Decode(out)
	// following line is used for debugging purposes.
	// _ = json.NewEncoder(os.Stdout).Encode(out)
	return res, decodeErr
}

// Error represents am Azure error.
type Error struct {
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return e.Message
}

func SanitizeBranchName(name string) string {
	if strings.Contains(name, "/") {
		return name
	}
	return "refs/heads/" + name
}

type repoObj struct {
	org     string
	project string
	name    string
}

func decodeRepo(repo string) (*repoObj, error) {
	repoTrimmed := strings.Trim(repo, "/")
	repoParts := strings.Split(repoTrimmed, "/")
	// test for correct number of values
	if len(repoParts) != 3 {
		return nil, fmt.Errorf("expected repository in form <organization>/<project>/<name>, but got %s", repoTrimmed)
	}

	// Test for empty values
	for _, s := range repoParts {
		if s == "" {
			return nil, fmt.Errorf("expected repository in form <organization>/<project>/<name>, but got %s", repoTrimmed)
		}
	}

	return &repoObj{
		org:     repoParts[0],
		project: repoParts[1],
		name:    repoParts[2],
	}, nil
}
