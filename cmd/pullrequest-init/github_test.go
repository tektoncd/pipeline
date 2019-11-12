package main

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestNewGitHubHandler(t *testing.T) {
	for _, url := range []string{
		"https://github.com/foo/bar/pull/1",
		"https://github.tekton.dev/foo/bar/pull/1",
	} {
		t.Run(url, func(t *testing.T) {
			h, err := NewGitHubHandler(context.Background(), zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller())).Sugar(), url)
			if err != nil {
				t.Fatalf("error creating GitHubHandler: %v", err)
			}
			if h.repo != repo {
				t.Fatalf("error unexpected repo: %v", h.repo)
			}
			if h.prNum != prNum {
				t.Fatalf("error unexpected PR number: %v", h.prNum)
			}
		})
	}
}
