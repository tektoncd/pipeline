/*
Copyright 2022 The Tekton Authors

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

package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type repository struct {
	url       string
	directory string
	revision  string
}

func resolveRepository(ctx context.Context, url, revision string) (*repository, func(), error) {
	urlParts := strings.Split(url, "/")
	repoName := urlParts[len(urlParts)-1]
	tmpDir, err := os.MkdirTemp("", repoName+"-*")
	if err != nil {
		return nil, func() {}, err
	}
	cleanupFunc := func() {
		os.RemoveAll(tmpDir)
	}

	repo := repository{
		url:       url,
		directory: tmpDir,
		revision:  revision,
	}

	_, err = repo.execGit(ctx, "clone", repo.url, tmpDir, "--depth=1", "--no-checkout")
	if err != nil {
		return nil, cleanupFunc, err
	}

	_, err = repo.execGit(ctx, "fetch", "origin", repo.revision, "--depth=1")
	if err != nil {
		return nil, cleanupFunc, err
	}

	_, err = repo.execGit(ctx, "checkout", "FETCH_HEAD")
	if err != nil {
		return nil, cleanupFunc, err
	}

	revisionSha, err := repo.execGit(ctx, "rev-list", "-n1", "HEAD")
	if err != nil {
		return nil, cleanupFunc, err
	}
	repo.revision = strings.TrimSpace(string(revisionSha))

	return &repo, cleanupFunc, nil
}

func (repo repository) execGit(ctx context.Context, args ...string) ([]byte, error) {
	gitCommangStr := ""
	if len(args) > 0 && args[0] != "clone" {
		// Just used for error message
		gitCommangStr = args[0]
		// If we're not cloning the repository, then we need to configure
		// which directory contains the cloned repository since  `cd`ing
		// into the repository directory is not concurrency-safe
		args = append([]string{"-C", repo.directory}, args...)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		msg := string(out)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			msg = string(exitErr.Stderr)
		}
		err = fmt.Errorf("git %s error: %s: %w", gitCommangStr, strings.TrimSpace(msg), err)
	}
	return out, err
}

func (repo repository) getFileContent(path string) ([]byte, error) {
	if _, err := os.Stat(repo.directory); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("repository clone no longer exists, used after cleaned? %w", err)
	}
	fileContents, err := os.ReadFile(filepath.Join(repo.directory, path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("file does not exist")
		}
		return nil, err
	}
	return fileContents, nil
}
