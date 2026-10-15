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
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestClone(t *testing.T) {
	type testCase struct {
		url       string
		username  string
		password  string
		expectErr string
	}

	testCases := map[string]testCase{
		"normal usage":           {url: "https://github.com/tektoncd/pipeline"},
		"normal usage with .git": {url: "https://github.com/tektoncd/pipeline.git"},
		"private repository":     {url: "https://github.com/tektoncd/not-a-repository.git"},
		"with crendentials":      {url: "https://github.com/tektoncd/not-a-repository.git", username: "fake", password: "fake"},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			executions := []*exec.Cmd{}

			executor := func(ctx context.Context, name string, args ...string) *exec.Cmd {
				args = append([]string{name}, args...)
				// Run the command as `echo` args to avoid side effects
				cmd := exec.CommandContext(ctx, "echo", args...)
				executions = append(executions, cmd)
				return cmd
			}

			mockCmdRemote := remote{url: test.url, username: test.username, password: test.password, cmdExecutor: executor}
			repo, cleanup, err := mockCmdRemote.clone(t.Context())
			defer cleanup()
			if test.expectErr != "" {
				if err.Error() != test.expectErr {
					t.Fatalf("Expected error %q but got %q", test.expectErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("Error cloning repository %q: %v", test.url, err)
				}
			}

			expectedEnv := []string{"GIT_TERMINAL_PROMPT=false"}
			expectedCmd := []string{"git", "-C", repo.directory}
			if test.username != "" {
				token := base64.StdEncoding.EncodeToString([]byte(test.username + ":" + test.password))
				expectedCmd = append(expectedCmd, "--config-env", "http.extraHeader=GIT_AUTH_HEADER")
				expectedEnv = append(expectedEnv, "GIT_AUTH_HEADER=Authorization: Basic "+token)
			}
			expectedCmd = append(expectedCmd, "clone", "--depth=1", "--no-checkout", "--filter=blob:none", "--sparse", "--", test.url, repo.directory)

			if len(executions) != 1 {
				t.Fatalf("Expected 1 command execution during cloning, got %d: %v", len(executions), executions)
			}

			cmd := executions[0]
			// Remove the `echo` prefix
			cmdParts := cmd.Args[1:]
			if !reflect.DeepEqual(cmdParts, expectedCmd) {
				t.Fatalf("Expected clone command to be %v but got %v", expectedCmd, cmdParts)
			}

			missingEnvVars := []string{}
			for _, v := range expectedEnv {
				if !slices.Contains(cmd.Environ(), v) {
					missingEnvVars = append(missingEnvVars, v)
				}
			}
			if len(missingEnvVars) > 0 {
				t.Fatalf("Clone command missing env vars %v. Got: %v", missingEnvVars, cmd.Environ())
			}
		})
	}
}

func TestCheckout(t *testing.T) {
	repoPath, revisions := createTestRepo(
		t,
		[]commitForRepo{
			{
				Filename: "README.md",
				Content:  "some content",
				Branch:   "non-main",
				Tag:      "1.0.0",
			},
			{
				Filename: "otherfile.yaml",
				Content:  "some data",
				Branch:   "to-be-deleted",
			},
		},
	)
	gitCmd := getGitCmd(t, repoPath)
	if err := gitCmd("checkout", "main").Run(); err != nil {
		t.Fatalf("cloud not checkout main branch after repo initialization: %v", err)
	}
	if err := gitCmd("branch", "-D", "to-be-deleted").Run(); err != nil {
		t.Fatalf("coun't delete branch to orphan commit: %v", err)
	}

	ctx := t.Context()

	type testCase struct {
		revision         string
		expectedRevision string
		expectErr        string
	}
	testCases := map[string]testCase{
		"revision is branch":          {revision: "non-main", expectedRevision: revisions[0]},
		"revision is tag":             {revision: "1.0.0", expectedRevision: revisions[0]},
		"revision is sha":             {revision: revisions[0], expectedRevision: revisions[0]},
		"revision is unreachable sha": {revision: revisions[1], expectedRevision: revisions[1]},
		"non-existent revision":       {revision: "fake-revision", expectErr: "git fetch error: fatal: couldn't find remote ref fake-revision: exit status 128"},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			repo, cleanup, err := remote{url: repoPath}.clone(ctx)
			defer cleanup()

			if err != nil {
				t.Fatalf("Error cloning repository %v", err)
			}

			err = repo.checkout(ctx, test.revision)
			if test.expectErr != "" {
				if err == nil {
					t.Fatal("Expected error checking out revision but got none")
				} else if err.Error() != test.expectErr {
					t.Fatalf("Expected error %q but got %q", test.expectErr, err)
				}
				return
			} else if err != nil {
				t.Fatalf("Error checking out revision: %v", err)
			}

			revision, err := repo.currentRevision(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if revision != test.expectedRevision {
				t.Fatalf("Expected revision to be %q but got %q", test.expectedRevision, revision)
			}
		})
	}
}

// TestCheckout_ArgumentInjection_CommandStructure uses a mock executor
// to verify that checkout() places the "--" separator before the
// revision argument, preventing git from interpreting a malicious
// revision (e.g. "--upload-pack=/bin/sh") as a flag.
//
// Regression test for GHSA-94jr-7pqp-xhcq.
func TestCheckout_ArgumentInjection_CommandStructure(t *testing.T) {
	testCases := []struct {
		name     string
		revision string
	}{
		{name: "upload-pack injection", revision: "--upload-pack=/bin/sh"},
		{name: "single dash flag", revision: "-v"},
		{name: "normal revision still works", revision: "main"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var allInvocations [][]string

			executor := func(ctx context.Context, name string, args ...string) *exec.Cmd {
				// Capture every invocation's full argument list.
				invocation := append([]string{name}, args...)
				allInvocations = append(allInvocations, invocation)
				// Use "echo" to avoid actually running git.
				return exec.CommandContext(ctx, "echo", invocation...)
			}

			repo := &repository{
				directory: t.TempDir(),
				executor:  executor,
			}

			_ = repo.checkout(t.Context(), tc.revision)

			// checkout() calls execGit twice: first "fetch", then "checkout".
			// We inspect the "fetch" invocation (first call).
			if len(allInvocations) == 0 {
				t.Fatal("expected at least one git invocation, got none")
			}
			fetchArgs := allInvocations[0]

			// Find the position of "--" and the revision in the fetch args.
			separatorIdx := -1
			revisionIdx := -1
			for i, arg := range fetchArgs {
				if arg == "--" {
					separatorIdx = i
				}
				if arg == tc.revision {
					revisionIdx = i
				}
			}

			if separatorIdx == -1 {
				t.Fatalf("expected '--' separator in git fetch args, got: %v", fetchArgs)
			}
			if revisionIdx == -1 {
				t.Fatalf("expected revision %q in git fetch args, got: %v", tc.revision, fetchArgs)
			}
			if revisionIdx < separatorIdx {
				t.Fatalf("revision %q appears before '--' separator (index %d < %d), "+
					"which means git could interpret it as a flag: %v",
					tc.revision, revisionIdx, separatorIdx, fetchArgs)
			}
		})
	}
}

// TestCheckout_ArgumentInjection_RealGit creates a real git repository
// and verifies that a malicious revision containing "--upload-pack"
// cannot execute a binary. With the "--" separator, git treats the
// value as a refspec (which doesn't exist) rather than a flag.
//
// Regression test for GHSA-94jr-7pqp-xhcq.
func TestCheckout_ArgumentInjection_RealGit(t *testing.T) {
	// Create a local git repo to serve as the remote.
	repoPath, _ := createTestRepo(t, []commitForRepo{
		{Filename: "task.yaml", Content: "apiVersion: tekton.dev/v1\nkind: Task"},
	})

	// Create a marker file that would be written if the exploit succeeds.
	markerFile := filepath.Join(t.TempDir(), "exploit-marker")

	// Create an exploit script that writes the marker file.
	exploitScript := filepath.Join(t.TempDir(), "exploit.sh")
	if err := os.WriteFile(exploitScript, []byte("#!/bin/sh\necho EXPLOITED > "+markerFile+"\n"), 0o700); err != nil {
		t.Fatalf("failed to create exploit script: %v", err)
	}

	ctx := t.Context()

	// Clone the repo (as the resolver would).
	repo, cleanup, err := remote{url: repoPath}.clone(ctx)
	if err != nil {
		t.Fatalf("failed to clone test repo: %v", err)
	}
	defer cleanup()

	// Attempt checkout with a malicious revision that tries to inject
	// --upload-pack. With the "--" separator, git should treat this as
	// a refspec and fail with "couldn't find remote ref".
	maliciousRevision := "--upload-pack=" + exploitScript
	err = repo.checkout(ctx, maliciousRevision)

	// We expect an error because the "refspec" doesn't exist.
	if err == nil {
		t.Fatal("expected checkout to fail with malicious revision, but it succeeded")
	}

	// The critical assertion: the exploit script must NOT have been executed.
	if _, statErr := os.Stat(markerFile); statErr == nil {
		t.Fatalf("SECURITY FAILURE: exploit script was executed! "+
			"The '--upload-pack' argument was interpreted as a git flag "+
			"instead of a refspec. Marker file exists at %s", markerFile)
	}
}

func TestGetFileContent(t *testing.T) {
	// Create a file outside any repo to simulate a sensitive target.
	sensitiveDir := t.TempDir()
	sensitiveFile := filepath.Join(sensitiveDir, "sa-token")
	if err := os.WriteFile(sensitiveFile, []byte("stolen-credential"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a real git repository with a tracked file.
	// Resolve the temp dir so filepath.Rel works on platforms where /tmp
	// is a symlink (e.g. macOS /tmp -> /private/tmp).
	repoDir, _ := createTestRepo(t, []commitForRepo{
		{Dir: "tasks", Filename: "example.yaml", Content: "valid content"},
	})
	// Add a symlink that escapes and commit it.
	gitCmd := getGitCmd(t, repoDir)
	if err := os.Symlink(sensitiveFile, filepath.Join(repoDir, "escape-link")); err != nil {
		t.Fatal(err)
	}
	if out, err := gitCmd("add", "escape-link").Output(); err != nil {
		t.Fatalf("git add symlink: %q: %v", out, err)
	}
	// Add a nested symlink escape.
	nestedDir := filepath.Join(repoDir, "subdir")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sensitiveFile, filepath.Join(nestedDir, "nested-link")); err != nil {
		t.Fatal(err)
	}
	if out, err := gitCmd("add", "subdir/nested-link").Output(); err != nil {
		t.Fatalf("git add nested symlink: %q: %v", out, err)
	}
	if out, err := gitCmd("commit", "-m", "add symlinks").Output(); err != nil {
		t.Fatalf("git commit: %q: %v", out, err)
	}

	repo := &repository{directory: repoDir}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name: "valid relative path",
			path: "tasks/example.yaml",
		},
		{
			name:    "path traversal with dot-dot",
			path:    "../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "path traversal to parent",
			path:    "../secret",
			wantErr: true,
		},
		{
			name:    "path traversal deeply nested",
			path:    "../../../../var/run/secrets/kubernetes.io/serviceaccount/token",
			wantErr: true,
		},
		{
			name:    "path traversal embedded",
			path:    "tasks/../../../../../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "non-existent file",
			path:    "does-not-exist.yaml",
			wantErr: true,
		},
		{
			name:    "symlink escaping repo directory",
			path:    "escape-link",
			wantErr: true,
		},
		{
			name:    "symlink in subdirectory escaping repo",
			path:    filepath.Join("subdir", "nested-link"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			content, err := repo.getFileContent(tc.path)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (content: %q)", string(content))
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestGetFileContent_SymlinkEscape_RealGitRepo creates a real git
// repository with a committed symlink that points outside the repo,
// clones it, checks out the revision, and verifies that getFileContent
// rejects the symlink path. This exercises the full clone → checkout →
// read flow with an actual git repository.
func TestGetFileContent_SymlinkEscape_RealGitRepo(t *testing.T) {
	// Create a sensitive file outside any repo to simulate a target.
	sensitiveDir := t.TempDir()
	sensitiveFile := filepath.Join(sensitiveDir, "sa-token")
	if err := os.WriteFile(sensitiveFile, []byte("stolen-credential"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a git repository with a normal file and a symlink escape.
	repoDir, _ := createTestRepo(t, []commitForRepo{
		{Filename: "task.yaml", Content: "apiVersion: tekton.dev/v1\nkind: Task"},
	})

	// Add a symlink that points to the sensitive file and commit it.
	gitCmd := getGitCmd(t, repoDir)
	symlinkPath := filepath.Join(repoDir, "escape-link")
	if err := os.Symlink(sensitiveFile, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}
	if out, err := gitCmd("add", "escape-link").Output(); err != nil {
		t.Fatalf("git add symlink failed: %q: %v", out, err)
	}
	if out, err := gitCmd("commit", "-m", "add symlink escape").Output(); err != nil {
		t.Fatalf("git commit symlink failed: %q: %v", out, err)
	}

	// Also add a symlink in a subdirectory.
	subdir := filepath.Join(repoDir, "configs")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	nestedSymlink := filepath.Join(subdir, "nested-escape")
	if err := os.Symlink(sensitiveFile, nestedSymlink); err != nil {
		t.Fatalf("failed to create nested symlink: %v", err)
	}
	if out, err := gitCmd("add", "configs/nested-escape").Output(); err != nil {
		t.Fatalf("git add nested symlink failed: %q: %v", out, err)
	}
	if out, err := gitCmd("commit", "-m", "add nested symlink escape").Output(); err != nil {
		t.Fatalf("git commit nested symlink failed: %q: %v", out, err)
	}

	// Clone the repo (as the resolver would) and checkout main.
	ctx := t.Context()
	repo, cleanup, err := remote{url: repoDir}.clone(ctx)
	if err != nil {
		t.Fatalf("failed to clone test repo: %v", err)
	}
	defer cleanup()

	if err := repo.checkout(ctx, "main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}

	// Verify a normal file can be read.
	content, err := repo.getFileContent("task.yaml")
	if err != nil {
		t.Fatalf("expected to read normal file, got error: %v", err)
	}
	if !contains(string(content), "tekton.dev") {
		t.Fatalf("unexpected content: %s", content)
	}

	// Verify the symlink escape is blocked.
	tests := []struct {
		name string
		path string
	}{
		{name: "top-level symlink escape", path: "escape-link"},
		{name: "nested symlink escape", path: "configs/nested-escape"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			content, err := repo.getFileContent(tc.path)
			if err == nil {
				t.Fatalf("symlink escape was NOT blocked — read %d bytes: %q", len(content), string(content))
			}
		})
	}
}

// TestClonePartial_ArgumentStructure uses a mock executor to verify the
// exact git argv for the filtered, sparse clone path: the cone derived
// from pathInRepo, and that a root-level path skips "sparse-checkout set"
// entirely since "--sparse" alone already materialises root-level files.
func TestClonePartial_ArgumentStructure(t *testing.T) {
	testCases := []struct {
		name            string
		pathInRepo      string
		expectSparseSet bool
		expectedDir     string
	}{
		{name: "nested path", pathInRepo: ".tekton/pipeline.yaml", expectSparseSet: true, expectedDir: ".tekton"},
		{name: "deeply nested path", pathInRepo: "a/b/c/pipeline.yaml", expectSparseSet: true, expectedDir: "a/b/c"},
		{name: "root-level path", pathInRepo: "pipeline.yaml", expectSparseSet: false},
		{name: "empty path", pathInRepo: "", expectSparseSet: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var invocations [][]string
			executor := func(ctx context.Context, name string, args ...string) *exec.Cmd {
				invocation := append([]string{name}, args...)
				invocations = append(invocations, invocation)
				return exec.CommandContext(ctx, "echo", invocation...)
			}

			r := remote{url: "https://example.invalid/repo.git", pathInRepo: tc.pathInRepo, cmdExecutor: executor}
			repo, cleanup, err := r.clone(t.Context())
			defer cleanup()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !repo.partial {
				t.Fatal("expected the partial, sparse clone to have succeeded")
			}

			cloneArgs := invocations[0]
			for _, want := range []string{"--filter=blob:none", "--sparse"} {
				if !slices.Contains(cloneArgs, want) {
					t.Fatalf("expected clone args to contain %q, got %v", want, cloneArgs)
				}
			}

			sawSparseSet := false
			for _, inv := range invocations[1:] {
				if !slices.Contains(inv, "sparse-checkout") {
					continue
				}
				sawSparseSet = true
				sepIdx, dirIdx := -1, -1
				for i, a := range inv {
					if a == "--" {
						sepIdx = i
					}
					if a == tc.expectedDir {
						dirIdx = i
					}
				}
				if sepIdx == -1 {
					t.Fatalf("expected '--' separator in sparse-checkout set args: %v", inv)
				}
				if dirIdx == -1 || dirIdx < sepIdx {
					t.Fatalf("expected dir %q after the '--' separator in sparse-checkout set args: %v", tc.expectedDir, inv)
				}
			}
			if sawSparseSet != tc.expectSparseSet {
				t.Fatalf("expected sparse-checkout set invoked=%v, got %v (invocations: %v)", tc.expectSparseSet, sawSparseSet, invocations)
			}
		})
	}
}

// TestClonePartial_FallbackOnFailure verifies that when the filtered,
// sparse clone attempt fails for any reason, clone() retries once using
// the original unfiltered sequence instead of failing the resolution.
// This is R2's highest-risk path: a server without uploadpack.allowFilter
// must keep working, not break.
func TestClonePartial_FallbackOnFailure(t *testing.T) {
	var invocations [][]string
	filteredAttempts := 0
	executor := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		invocation := append([]string{name}, args...)
		invocations = append(invocations, invocation)
		if slices.Contains(invocation, "clone") && slices.Contains(invocation, "--filter=blob:none") {
			filteredAttempts++
			// Simulate a server that rejects the filtered clone, every
			// time. Exit non-zero without touching the target directory,
			// exactly like a real failed git-clone would.
			return exec.CommandContext(ctx, "false")
		}
		return exec.CommandContext(ctx, "echo", invocation...)
	}

	r := remote{url: "https://example.invalid/repo.git", pathInRepo: "sub/dir/file.yaml", cmdExecutor: executor}
	repo, cleanup, err := r.clone(t.Context())
	defer cleanup()
	if err != nil {
		t.Fatalf("expected clone to succeed via fallback, got error: %v", err)
	}
	if repo.partial {
		t.Fatal("expected repo.partial to be false after falling back to the unfiltered clone")
	}
	if filteredAttempts != filteredOperationAttempts {
		t.Fatalf("expected %d filtered clone attempts before falling back, got %d", filteredOperationAttempts, filteredAttempts)
	}

	cloneInvocations := 0
	for _, inv := range invocations {
		if slices.Contains(inv, "clone") {
			cloneInvocations++
		}
		if slices.Contains(inv, "sparse-checkout") {
			t.Fatalf("did not expect sparse-checkout set to run when every filtered clone attempt failed: %v", invocations)
		}
	}
	if cloneInvocations != filteredOperationAttempts+1 {
		t.Fatalf("expected %d clone invocations (%d failed filtered attempts + fallback), got %d: %v", filteredOperationAttempts+1, filteredOperationAttempts, cloneInvocations, invocations)
	}

	fallbackArgs := invocations[len(invocations)-1]
	for _, unwanted := range []string{"--filter=blob:none", "--sparse"} {
		if slices.Contains(fallbackArgs, unwanted) {
			t.Fatalf("fallback clone must not include %q, got %v", unwanted, fallbackArgs)
		}
	}
}

// TestClonePartial_RetriesBeforeFallback verifies that a single transient
// failure of the filtered clone does not immediately escalate to an
// expensive full clone: the same filtered attempt is retried once first,
// and if that retry succeeds, no fallback happens at all. This is the
// large-repo-on-a-flaky-connection case: a one-off network blip should not
// guarantee the costly unfiltered path that this change exists to avoid.
func TestClonePartial_RetriesBeforeFallback(t *testing.T) {
	var invocations [][]string
	filteredAttempts := 0
	executor := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		invocation := append([]string{name}, args...)
		invocations = append(invocations, invocation)
		if slices.Contains(invocation, "clone") && slices.Contains(invocation, "--filter=blob:none") {
			filteredAttempts++
			if filteredAttempts == 1 {
				// First attempt fails transiently; every later attempt
				// (including sparse-checkout set) succeeds.
				return exec.CommandContext(ctx, "false")
			}
		}
		return exec.CommandContext(ctx, "echo", invocation...)
	}

	r := remote{url: "https://example.invalid/repo.git", cmdExecutor: executor}
	repo, cleanup, err := r.clone(t.Context())
	defer cleanup()
	if err != nil {
		t.Fatalf("expected the retried partial clone to succeed, got error: %v", err)
	}
	if !repo.partial {
		t.Fatal("expected repo.partial to remain true — the retry should have avoided any fallback")
	}
	if filteredAttempts != 2 {
		t.Fatalf("expected exactly 2 filtered clone attempts (1 failed + 1 retry), got %d", filteredAttempts)
	}

	cloneInvocations := 0
	for _, inv := range invocations {
		if slices.Contains(inv, "clone") {
			cloneInvocations++
			if !slices.Contains(inv, "--filter=blob:none") || !slices.Contains(inv, "--sparse") {
				t.Fatalf("expected every clone invocation to still be filtered — no fallback should have occurred: %v", inv)
			}
		}
	}
	if cloneInvocations != 2 {
		t.Fatalf("expected exactly 2 clone invocations (no fallback clone), got %d: %v", cloneInvocations, invocations)
	}
}

// TestCheckout_FallbackOnFilteredFetchFailure verifies that a failure in
// the filtered fetch — not just the initial filtered clone — also triggers
// the fallback to an unfiltered sequence. clone() and checkout() are
// separate git invocations against separate connections, so a server (or
// network) that tolerates --filter at clone time is not guaranteed to at
// fetch time too; checkout() must not simply propagate that failure.
func TestCheckout_FallbackOnFilteredFetchFailure(t *testing.T) {
	var invocations [][]string
	filteredFetchAttempts := 0
	executor := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		invocation := append([]string{name}, args...)
		invocations = append(invocations, invocation)
		if slices.Contains(invocation, "fetch") && slices.Contains(invocation, "--filter=blob:none") {
			filteredFetchAttempts++
			// Simulate a server that accepted the filtered clone but
			// rejects (or fails) the filtered fetch, every time — a
			// distinct connection/negotiation from the clone.
			return exec.CommandContext(ctx, "false")
		}
		return exec.CommandContext(ctx, "echo", invocation...)
	}

	r := remote{url: "https://example.invalid/repo.git", pathInRepo: "sub/dir/file.yaml", cmdExecutor: executor}
	repo, cleanup, err := r.clone(t.Context())
	defer cleanup()
	if err != nil {
		t.Fatalf("expected the initial partial clone to succeed, got error: %v", err)
	}
	if !repo.partial {
		t.Fatal("expected repo.partial to be true after a successful partial clone")
	}

	if err := repo.checkout(t.Context(), "main"); err != nil {
		t.Fatalf("expected checkout to succeed via fallback, got error: %v", err)
	}
	if repo.partial {
		t.Fatal("expected repo.partial to be false after falling back to an unfiltered clone/fetch")
	}
	if filteredFetchAttempts != filteredOperationAttempts {
		t.Fatalf("expected %d filtered fetch attempts before falling back, got %d", filteredOperationAttempts, filteredFetchAttempts)
	}

	var cloneInvocations, fetchInvocations [][]string
	for _, inv := range invocations {
		if slices.Contains(inv, "clone") {
			cloneInvocations = append(cloneInvocations, inv)
		}
		if slices.Contains(inv, "fetch") {
			fetchInvocations = append(fetchInvocations, inv)
		}
	}
	if len(cloneInvocations) != 2 {
		t.Fatalf("expected 2 clone invocations (initial partial clone + fallback reclone), got %d: %v", len(cloneInvocations), invocations)
	}
	if len(fetchInvocations) != filteredOperationAttempts+1 {
		t.Fatalf("expected %d fetch invocations (%d failed filtered fetch attempts + unfiltered fallback), got %d: %v", filteredOperationAttempts+1, filteredOperationAttempts, len(fetchInvocations), invocations)
	}

	// The fallback reclone (second clone invocation) must be the plain,
	// unfiltered sequence — not a repeat of the filtered attempt.
	fallbackCloneArgs := cloneInvocations[1]
	for _, unwanted := range []string{"--filter=blob:none", "--sparse"} {
		if slices.Contains(fallbackCloneArgs, unwanted) {
			t.Fatalf("fallback reclone must not include %q, got %v", unwanted, fallbackCloneArgs)
		}
	}

	fallbackFetchArgs := fetchInvocations[len(fetchInvocations)-1]
	if slices.Contains(fallbackFetchArgs, "--filter=blob:none") {
		t.Fatalf("fallback fetch must not include --filter=blob:none, got %v", fallbackFetchArgs)
	}
}

// TestCheckout_RetriesFetchBeforeFallback mirrors
// TestClonePartial_RetriesBeforeFallback for the fetch stage: a single
// transient failure of the filtered fetch is retried once before checkout()
// escalates to a full, unfiltered clone.
func TestCheckout_RetriesFetchBeforeFallback(t *testing.T) {
	var invocations [][]string
	filteredFetchAttempts := 0
	executor := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		invocation := append([]string{name}, args...)
		invocations = append(invocations, invocation)
		if slices.Contains(invocation, "fetch") && slices.Contains(invocation, "--filter=blob:none") {
			filteredFetchAttempts++
			if filteredFetchAttempts == 1 {
				return exec.CommandContext(ctx, "false")
			}
		}
		return exec.CommandContext(ctx, "echo", invocation...)
	}

	r := remote{url: "https://example.invalid/repo.git", cmdExecutor: executor}
	repo, cleanup, err := r.clone(t.Context())
	defer cleanup()
	if err != nil {
		t.Fatalf("failed to clone: %v", err)
	}

	if err := repo.checkout(t.Context(), "main"); err != nil {
		t.Fatalf("expected the retried fetch to succeed, got error: %v", err)
	}
	if !repo.partial {
		t.Fatal("expected repo.partial to remain true — the retry should have avoided any fallback")
	}
	if filteredFetchAttempts != 2 {
		t.Fatalf("expected exactly 2 filtered fetch attempts (1 failed + 1 retry), got %d", filteredFetchAttempts)
	}

	cloneInvocations := 0
	for _, inv := range invocations {
		if slices.Contains(inv, "clone") {
			cloneInvocations++
		}
	}
	if cloneInvocations != 1 {
		t.Fatalf("expected exactly 1 clone invocation (no fallback reclone), got %d: %v", cloneInvocations, invocations)
	}
}

// TestClonePartial_SparseCheckoutRestrictsTree exercises the real git
// binary (via a file:// remote, which — unlike a plain local path — goes
// through the same upload-pack negotiation as a network clone) and
// verifies that a sibling directory outside pathInRepo's cone is never
// materialised, while root-level files still are.
func TestClonePartial_SparseCheckoutRestrictsTree(t *testing.T) {
	repoDir, _ := createTestRepo(t, []commitForRepo{
		{Dir: "tasks", Filename: "example.yaml", Content: "wanted"},
		{Dir: "other", Filename: "big.bin", Content: "not wanted"},
	})

	ctx := t.Context()
	repo, cleanup, err := remote{url: "file://" + repoDir, pathInRepo: "tasks/example.yaml"}.clone(ctx)
	defer cleanup()
	if err != nil {
		t.Fatalf("failed to clone: %v", err)
	}
	if !repo.partial {
		t.Fatal("expected the file:// clone to take the partial path")
	}
	if err := repo.checkout(ctx, "main"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}

	content, err := repo.getFileContent("tasks/example.yaml")
	if err != nil {
		t.Fatalf("expected the requested file to be present: %v", err)
	}
	if string(content) != "wanted" {
		t.Fatalf("expected content %q, got %q", "wanted", string(content))
	}
	if _, err := repo.getFileContent("README"); err != nil {
		t.Fatalf("expected root-level files to still be present in cone mode: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo.directory, "other")); !os.IsNotExist(err) {
		t.Fatalf("expected sibling directory 'other' to be absent from the working tree, stat returned: %v", err)
	}
}

// TestGetFileContent_ErrorParity_MissingSparseDirectory verifies that when
// pathInRepo names a directory that doesn't exist in the repository,
// resolution still fails with the same "file does not exist" error as
// before this change. "git sparse-checkout set" succeeds silently on a
// missing cone and yields an empty working tree, so this isn't automatic —
// it depends on getFileContent's existing os.ErrNotExist handling.
func TestGetFileContent_ErrorParity_MissingSparseDirectory(t *testing.T) {
	repoDir, _ := createTestRepo(t, []commitForRepo{
		{Dir: "tasks", Filename: "example.yaml", Content: "valid content"},
	})

	ctx := t.Context()
	repo, cleanup, err := remote{url: "file://" + repoDir, pathInRepo: "does-not-exist/pipeline.yaml"}.clone(ctx)
	defer cleanup()
	if err != nil {
		t.Fatalf("failed to clone: %v", err)
	}
	if !repo.partial {
		t.Fatal("expected the file:// clone to take the partial path")
	}
	if err := repo.checkout(ctx, "main"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}

	_, err = repo.getFileContent("does-not-exist/pipeline.yaml")
	if err == nil {
		t.Fatal("expected an error resolving a file under a missing sparse directory, got nil")
	}
	if err.Error() != "file does not exist" {
		t.Fatalf("expected error %q, got %q", "file does not exist", err.Error())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
