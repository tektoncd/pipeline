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
	"os/exec"
	"reflect"
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
			repo, cleanup, err := mockCmdRemote.clone(context.Background())
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
				token := base64.URLEncoding.EncodeToString([]byte(test.username + ":" + test.password))
				expectedCmd = append(expectedCmd, "--config-env", "http.extraHeader=GIT_AUTH_HEADER")
				expectedEnv = append(expectedEnv, "GIT_AUTH_HEADER=Authorization=Basic "+token)
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
			if !reflect.DeepEqual(cmd.Env, expectedEnv) {
				t.Fatalf("Expected clone command env vars to be %v but got %v", expectedEnv, cmd.Env)
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

	ctx := context.Background()

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
