/*
Copyright 2025 The Tekton Authors

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
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type cmdExecutor = func(context.Context, string, ...string) *exec.Cmd

type remote struct {
	url         string
	username    string
	password    string
	cmdExecutor cmdExecutor
}

func (r remote) clone(ctx context.Context) (*repository, func(), error) {
	urlParts := strings.Split(r.url, "/")
	repoName := urlParts[len(urlParts)-1]
	tmpDir, err := os.MkdirTemp("", repoName+"-*")
	if err != nil {
		return nil, func() {}, err
	}
	cleanupFunc := func() {
		os.RemoveAll(tmpDir)
	}

	repo := &repository{
		url:       r.url,
		username:  r.username,
		password:  r.password,
		directory: tmpDir,
		executor:  r.cmdExecutor,
	}

	_, err = repo.execGit(ctx, "clone", repo.url, tmpDir, "--depth=1", "--no-checkout")
	if err != nil {
		if strings.Contains(err.Error(), "could not read Username") {
			err = errors.New("clone error: authentication required")
		}
		return nil, cleanupFunc, err
	}
	return repo, cleanupFunc, nil
}

type repository struct {
	url       string
	username  string
	password  string
	directory string
	executor  cmdExecutor
}

func (repo *repository) currentRevision(ctx context.Context) (string, error) {
	revisionSha, err := repo.execGit(ctx, "rev-list", "-n1", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(revisionSha)), nil
}

func (repo *repository) checkout(ctx context.Context, revision string) error {
	_, err := repo.execGit(ctx, "fetch", "origin", revision, "--depth=1")
	if err != nil {
		return err
	}

	_, err = repo.execGit(ctx, "checkout", "FETCH_HEAD")
	if err != nil {
		return err
	}

	return nil
}

func (repo *repository) execGit(ctx context.Context, subCmd string, args ...string) ([]byte, error) {
	if repo.executor == nil {
		repo.executor = exec.CommandContext
	}

	args = append([]string{subCmd}, args...)

	// We need to configure  which directory contains the cloned repository since `cd`ing
	// into the repository directory is not concurrency-safe
	configArgs := []string{"-C", repo.directory}

	env := []string{"GIT_TERMINAL_PROMPT=false"}
	// NOTE: Since this is only HTTP basic auth, authentication is only supported for http
	// cloning, while unauthenticated cloning is supported for any other protocol supported
	// by git which doesn't require authentication.
	if repo.username != "" && repo.password != "" {
		token := base64.URLEncoding.EncodeToString([]byte(repo.username + ":" + repo.password))
		env = append(
			env,
			"GIT_AUTH_HEADER=Authorization: Basic "+token,
		)
		configArgs = append(configArgs, "--config-env", "http.extraHeader=GIT_AUTH_HEADER")
	}
	cmd := repo.executor(ctx, "git", append(configArgs, args...)...)
	cmd.Env = append(cmd.Env, env...)

	out, err := cmd.Output()
	if err != nil {
		msg := string(out)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			msg = string(exitErr.Stderr)
		}
		err = fmt.Errorf("git %s error: %s: %w", subCmd, strings.TrimSpace(msg), err)
	}
	return out, err
}

func (repo *repository) getFileContent(path string) ([]byte, error) {
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
