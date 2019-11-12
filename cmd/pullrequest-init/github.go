package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"golang.org/x/oauth2"

	"github.com/jenkins-x/go-scm/scm/driver/github"
	"go.uber.org/zap"
)

func NewGitHubHandler(ctx context.Context, logger *zap.SugaredLogger, raw string) (*Handler, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	split := strings.Split(u.Path, "/")
	if len(split) < 5 {
		return nil, fmt.Errorf("could not determine PR from URL: %v", raw)
	}
	owner, repo, pr := split[1], split[2], split[4]
	prNumber, err := strconv.Atoi(pr)
	if err != nil {
		return nil, fmt.Errorf("error parsing PR number: %s", pr)
	}

	client := github.NewDefault()
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		hc := oauth2.NewClient(ctx, ts)
		client.Client = hc
	}
	ownerRepo := fmt.Sprintf("%s/%s", owner, repo)
	return NewHandler(logger, client, ownerRepo, prNumber), nil
}
