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
	"path"
	"path/filepath"
	"strings"

	"knative.dev/pkg/logging"
)

type cmdExecutor = func(context.Context, string, ...string) *exec.Cmd

type remote struct {
	url         string
	username    string
	password    string
	pathInRepo  string
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
		url:        r.url,
		username:   r.username,
		password:   r.password,
		pathInRepo: r.pathInRepo,
		directory:  tmpDir,
		executor:   r.cmdExecutor,
	}

	err = repo.clonePartialWithRetry(ctx)
	if err == nil {
		logging.FromContext(ctx).Debugf("git resolver: used partial, sparse clone for %q", r.url)
		return repo, cleanupFunc, nil
	}
	logging.FromContext(ctx).Debugf("git resolver: partial clone failed after retry (%v), falling back to a full clone", err)

	if err := repo.cloneFull(ctx); err != nil {
		return nil, cleanupFunc, err
	}
	return repo, cleanupFunc, nil
}

// filteredOperationAttempts bounds the retries in clonePartialWithRetry and
// fetchAndCheckoutWithRetry. A single transient network failure — as
// distinct from a genuine capability rejection by the server — should not
// immediately escalate to a full, unfiltered clone of what may be a very
// large repository: retrying the same cheap filtered operation once is
// strictly better for that case, and no worse for the "server doesn't
// support this at all" case, which fails the same way on both attempts.
// Deliberately not configurable yet — see local-validation.md and TODO.md
// for the open question of whether upstream wants this tunable.
const filteredOperationAttempts = 2

// clonePartialWithRetry attempts a partial, sparse clone that fetches only
// the directory containing pathInRepo instead of the whole commit, retrying
// once (see filteredOperationAttempts) before giving up. This requires the
// server to support uploadpack.allowFilter; any failure here — which is not
// limited to that case — is left for the caller to fall back from. git's
// error text is not a stable interface, so it is never pattern-matched to
// decide whether to retry or fall back.
func (repo *repository) clonePartialWithRetry(ctx context.Context) error {
	var err error
	for attempt := 1; attempt <= filteredOperationAttempts; attempt++ {
		if err = repo.clonePartial(ctx); err == nil {
			return nil
		}
		if attempt < filteredOperationAttempts {
			logging.FromContext(ctx).Debugf("git resolver: partial clone attempt %d/%d failed (%v), retrying", attempt, filteredOperationAttempts, err)
			if resetErr := repo.resetDirectory(); resetErr != nil {
				return resetErr
			}
		}
	}
	return err
}

// resetDirectory removes and recreates repo.directory so a fresh clone
// attempt has the empty target directory "git clone" requires, and marks
// repo.partial false as soon as the previous clone's content is gone —
// even if the subsequent MkdirAll fails, repo.partial must not claim a
// partial clone that no longer exists on disk.
func (repo *repository) resetDirectory() error {
	if err := os.RemoveAll(repo.directory); err != nil {
		return err
	}
	repo.partial = false
	return os.MkdirAll(repo.directory, 0o755)
}

// cloneFull resets repo.directory and performs the original, unfiltered
// clone sequence. It is the fallback target both when the initial partial
// clone fails and — via checkout() — when a filtered fetch fails after an
// otherwise-successful partial clone: the two are separate git invocations
// against separate connections, so a server (or network) that tolerates
// --filter at clone time is not guaranteed to at fetch time either.
func (repo *repository) cloneFull(ctx context.Context) error {
	if err := repo.resetDirectory(); err != nil {
		return err
	}

	// The "--" separator ensures that repo.url is always interpreted as
	// a repository path, never as a flag — even if it starts with "-".
	_, err := repo.execGit(ctx, "clone", "--depth=1", "--no-checkout", "--", repo.url, repo.directory)
	if err != nil {
		if strings.Contains(err.Error(), "could not read Username") {
			err = errors.New("clone error: authentication required")
		}
		return err
	}
	return nil
}

// clonePartial performs one attempt of the filtered, sparse clone. See
// clonePartialWithRetry, its only caller.
func (repo *repository) clonePartial(ctx context.Context) error {
	if _, err := repo.execGit(ctx, "clone", "--depth=1", "--no-checkout", "--filter=blob:none", "--sparse", "--", repo.url, repo.directory); err != nil {
		return err
	}

	// A root-level pathInRepo needs no cone at all: "--sparse" alone
	// already materialises root-level files in cone mode.
	if dir := path.Dir(repo.pathInRepo); dir != "." {
		if _, err := repo.execGit(ctx, "sparse-checkout", "set", "--", dir); err != nil {
			return err
		}
	}

	repo.partial = true
	return nil
}

type repository struct {
	url        string
	username   string
	password   string
	pathInRepo string
	partial    bool
	directory  string
	executor   cmdExecutor
}

func (repo *repository) currentRevision(ctx context.Context) (string, error) {
	revisionSha, err := repo.execGit(ctx, "rev-list", "-n1", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(revisionSha)), nil
}

func (repo *repository) checkout(ctx context.Context, revision string) error {
	err := repo.fetchAndCheckoutWithRetry(ctx, revision)
	if err == nil {
		return nil
	}
	if !repo.partial {
		// Already on the unfiltered sequence — nothing left to fall back to.
		return err
	}
	logging.FromContext(ctx).Debugf("git resolver: filtered fetch/checkout failed after retry (%v), falling back to a full clone", err)

	if fallbackErr := repo.cloneFull(ctx); fallbackErr != nil {
		return fallbackErr
	}
	return repo.fetchAndCheckoutWithRetry(ctx, revision)
}

// fetchAndCheckoutWithRetry retries a failed fetch/checkout once (see
// filteredOperationAttempts) before returning control to checkout(), which
// decides whether to escalate to a full unfiltered clone. Retrying applies
// whether or not repo.partial is set, since a flaky connection can affect
// the unfiltered fetch (after a fallback, or when the clone itself already
// used the unfiltered path) just as much as the filtered one.
func (repo *repository) fetchAndCheckoutWithRetry(ctx context.Context, revision string) error {
	var err error
	for attempt := 1; attempt <= filteredOperationAttempts; attempt++ {
		if err = repo.fetchAndCheckout(ctx, revision); err == nil {
			return nil
		}
		if attempt < filteredOperationAttempts {
			logging.FromContext(ctx).Debugf("git resolver: fetch/checkout attempt %d/%d failed (%v), retrying", attempt, filteredOperationAttempts, err)
		}
	}
	return err
}

// fetchAndCheckout performs one attempt of the fetch and checkout. See
// fetchAndCheckoutWithRetry, its only caller.
func (repo *repository) fetchAndCheckout(ctx context.Context, revision string) error {
	fetchArgs := []string{"origin", "--depth=1"}
	if repo.partial {
		fetchArgs = append(fetchArgs, "--filter=blob:none")
	}
	// The "--" separator ensures that 'revision' is always interpreted as
	// a refspec, never as a flag. Without it, a revision like
	// "--upload-pack=/path/to/binary" would be parsed as the
	// --upload-pack flag by git, enabling argument injection.
	fetchArgs = append(fetchArgs, "--", revision)
	_, err := repo.execGit(ctx, "fetch", fetchArgs...)
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

	// We need to configure which directory contains the cloned repository since `cd`ing
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
	cmd.Env = append(cmd.Environ(), env...)

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

func (repo *repository) getFileContent(givenPath string) ([]byte, error) {
	if _, err := os.Stat(repo.directory); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("repository clone no longer exists, used after cleaned? %w", err)
	}

	// Resolve repo.directory itself so that filepath.Rel produces correct
	// results on platforms where the temp directory is a symlink (e.g.
	// macOS /tmp -> /private/tmp).
	repoDir, err := filepath.EvalSymlinks(repo.directory)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve repository directory: %w", err)
	}

	absPath, err := filepath.Abs(filepath.Join(repoDir, givenPath))
	if err != nil {
		return nil, err
	}

	// Resolve symlinks so that in-repo symlinks work correctly while
	// symlinks that escape the repo are caught by the containment check.
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("file does not exist")
		}
		return nil, err
	}

	absPath, err = filepath.Abs(resolvedPath)
	if err != nil {
		return nil, err
	}

	relativePath, err := filepath.Rel(repoDir, absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve relative path: %w", err)
	}

	// Detect path traversal attempts — the relative path should never
	// start with ".." after symlink resolution. Log a specific message
	// so administrators can set up alerts for attempted exploits.
	if containsDotDot(relativePath) {
		return nil, fmt.Errorf("path %q attempts to escape the repository directory (possible path traversal attack)", givenPath)
	}

	fileContents, err := os.ReadFile(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("file does not exist")
		}
		return nil, err
	}
	return fileContents, nil
}
