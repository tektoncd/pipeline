/*
Copyright 2018 The Knative Authors

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
package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/google/go-github/github"
	"github.com/knative/pkg/logging"
	"golang.org/x/oauth2"
)

var (
	prURL = flag.String("url", "", "The url of the pull request to initialize.")
	path  = flag.String("path", "", "Path of directory under which PR will be copied")
	mode  = flag.String("mode", "download", "Whether to operate in download or upload mode")
)

var (
	githubToken string
)

type Comment struct {
	Author string
	Text   string
	ID     int64
}

type Label struct {
	Text string
}

func main() {
	flag.Parse()
	logger, _ := logging.NewLogger("", "pullrequest-init")
	defer logger.Sync()

	githubToken = os.Getenv("GITHUBOAUTHTOKEN")
	githubToken = strings.TrimSuffix(githubToken, "\n")
	var hc *http.Client
	if githubToken != "" {
		logger.Info("Using oauth token from env var GITHUBOAUTHTOKEN.")
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: githubToken},
		)
		hc = oauth2.NewClient(context.Background(), ts)
	}
	// Parse the URL to figure out the provider
	u, err := url.Parse(*prURL)
	if err != nil {
		logger.Fatalf("error parsing url: %s", prURL)
	}

	split := strings.Split(u.Path, "/")
	if len(split) != 5 {
		logger.Fatalf("could not determine PR from URL: %s", *prURL)
	}
	owner, repo, pr := split[1], split[2], split[4]
	prNumber, err := strconv.Atoi(pr)
	if err != nil {
		logger.Fatalf("error parsing PR number: %s", pr)
	}

	client := github.NewClient(hc)

	switch *mode {
	case "download":
		logger.Info("RUNNING DOWNLOAD!")
		if err := handleDownload(logger, client, *path, owner, repo, prNumber); err != nil {
			logger.Fatal(err)
		}
	case "upload":
		logger.Info("RUNNING UPLOAD!")
		if err := handleUpload(logger, client, *path, owner, repo, prNumber); err != nil {
			logger.Fatal(err)
		}
	}
}

func handleDownload(logger *zap.SugaredLogger, client *github.Client, path, owner, repo string, pr int) error {
	ctx := context.Background()

	labelsPath := filepath.Join(path, "labels")
	commentsPath := filepath.Join(path, "comments")
	for _, p := range []string{commentsPath} {
		if err := os.MkdirAll(p, 0755); err != nil {
			return err
		}
	}

	// Labels
	gpr, _, err := client.PullRequests.Get(ctx, owner, repo, pr)
	if err != nil {
		return err
	}

	labels := []Label{}
	for _, l := range gpr.Labels {
		labels = append(labels, Label{
			Text: *l.Name,
		})
	}
	b, err := json.Marshal(labels)
	if err != nil {
		return err
	}
	logger.Infof("Writing labels to file: %s", labelsPath)
	if err := ioutil.WriteFile(labelsPath, b, 0644); err != nil {
		return err
	}

	// Comments
	ic, _, err := client.Issues.ListComments(ctx, owner, repo, pr, nil)
	for i, c := range ic {
		comment := Comment{
			Author: *c.User.Login,
			Text:   *c.Body,
			ID:     *c.ID,
		}

		b, err := json.Marshal(comment)
		if err != nil {
			return err
		}
		commentPath := filepath.Join(commentsPath, strconv.Itoa(i))
		logger.Infof("Writing comment to file: %s", commentPath)
		if err := ioutil.WriteFile(commentPath, b, 0644); err != nil {
			return err
		}
	}
	return nil
}

func handleUpload(logger *zap.SugaredLogger, client *github.Client, path, owner, repo string, pr int) error {
	// Sync labels first
	ctx := context.Background()
	logger.Infof("Syncing path: %s to pr %d", path, pr)
	labelsPath := filepath.Join(path, "labels")

	b, err := ioutil.ReadFile(labelsPath)
	if err != nil {
		return err
	}
	labels := []Label{}
	if err := json.Unmarshal(b, &labels); err != nil {
		return err
	}

	labelNames := []string{}
	for _, l := range labels {
		labelNames = append(labelNames, l.Text)
	}

	logger.Infof("Setting labels for PR %d to %v", pr, labelNames)
	if _, _, err := client.Issues.ReplaceLabelsForIssue(ctx, owner, repo, pr, labelNames); err != nil {
		return err
	}

	// Now sync comments.
	commentsPath := filepath.Join(path, "comments")
	desiredComments := map[int64]Comment{}

	fis, err := ioutil.ReadDir(commentsPath)
	if err != nil {
		return err
	}

	for _, fi := range fis {
		p := filepath.Join(commentsPath, fi.Name())
		b, err := ioutil.ReadFile(p)
		if err != nil {
			return err
		}
		c := Comment{}
		if err := json.Unmarshal(b, &c); err != nil {
			return err
		}
		desiredComments[c.ID] = c
	}
	logger.Infof("Setting comments for PR %d to: %v", pr, desiredComments)

	existingComments, _, err := client.Issues.ListComments(ctx, owner, repo, pr, nil)
	if err != nil {
		return err
	}

	for _, ec := range existingComments {
		dc, ok := desiredComments[*ec.ID]
		if !ok {
			// Delete
			logger.Infof("Deleting comment %d for PR %d", *ec.ID, pr)
			if _, err := client.Issues.DeleteComment(ctx, owner, repo, *ec.ID); err != nil {
				return err
			}
		} else if dc.Text != *ec.Body {
			//Update
			newComment := github.IssueComment{
				Body: &dc.Text,
				User: ec.User,
			}
			logger.Infof("Updating comment %d for PR %d to %s", *ec.ID, pr, dc.Text)
			if _, _, err := client.Issues.EditComment(ctx, owner, repo, *ec.ID, &newComment); err != nil {
				return err
			}
		}
		// Delete to track new comments.
		delete(desiredComments, *ec.ID)
	}

	for _, dc := range desiredComments {
		newComment := github.IssueComment{
			Body: &dc.Text,
		}
		logger.Infof("Creating comment %s for PR %d", dc.Text, pr)
		if _, _, err := client.Issues.CreateComment(ctx, owner, repo, pr, &newComment); err != nil {
			return err
		}
	}
	return nil
}
