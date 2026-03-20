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
			expectedCmd = append(expectedCmd, "clone", test.url, repo.directory, "--depth=1", "--no-checkout")

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
