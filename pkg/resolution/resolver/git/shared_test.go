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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "knative.dev/pkg/system/testing"
)

const defaultBranch string = "main"

// withTemporaryGitConfig resets the .gitconfig for the duration of the test.
func withTemporaryGitConfig(t *testing.T) {
	t.Helper()
	gitConfigDir := t.TempDir()
	key := "GIT_CONFIG_GLOBAL"
	t.Setenv(key, filepath.Join(gitConfigDir, "config"))
}

func getGitCmd(t *testing.T, dir string) func(...string) *exec.Cmd {
	t.Helper()
	withTemporaryGitConfig(t)
	return func(args ...string) *exec.Cmd {
		args = append(
			[]string{
				"-C", dir,
				"-c", "user.email=test@test.com",
				"-c", "user.name=PipelinesTests",
			},
			args...)
		return exec.Command("git", args...)
	}
}

// createTestRepo is used to instantiate a local test repository with the desired commits.
func createTestRepo(t *testing.T, commits []commitForRepo) (string, []string) {
	t.Helper()
	commitSHAs := []string{}

	tempDir := t.TempDir()
	gitCmd := getGitCmd(t, tempDir)
	err := gitCmd("init", "-b", "main").Run()
	if err != nil {
		t.Fatalf("couldn't create test repo: %v", err)
	}

	writeAndCommitToTestRepo(t, tempDir, "", "README", []byte("This is a test"))

	hashesByBranch := make(map[string][]string)

	// Iterate over the commits and add them.
	for _, cmt := range commits {
		branch := cmt.Branch
		if branch == "" {
			branch = defaultBranch
		}

		// If we're given a revision, check out that revision.
		checkoutCmd := gitCmd("checkout")
		if _, ok := hashesByBranch[branch]; !ok && branch != defaultBranch {
			checkoutCmd.Args = append(checkoutCmd.Args, "-b")
		}
		checkoutCmd.Args = append(checkoutCmd.Args, branch)

		if err := checkoutCmd.Run(); err != nil {
			t.Fatalf("couldn't do checkout of %s: %v", branch, err)
		}

		hash := writeAndCommitToTestRepo(t, tempDir, cmt.Dir, cmt.Filename, []byte(cmt.Content))
		commitSHAs = append(commitSHAs, hash)

		if _, ok := hashesByBranch[branch]; !ok {
			hashesByBranch[branch] = []string{hash}
		} else {
			hashesByBranch[branch] = append(hashesByBranch[branch], hash)
		}

		if cmt.Tag != "" {
			err = gitCmd("tag", cmt.Tag).Run()
			if err != nil {
				t.Fatalf("couldn't add tag for %s: %v", cmt.Tag, err)
			}
		}
	}

	return tempDir, commitSHAs
}

// commitForRepo provides the directory, filename, content and revision for a test commit.
type commitForRepo struct {
	Dir      string
	Filename string
	Content  string
	Branch   string
	Tag      string
}

func writeAndCommitToTestRepo(t *testing.T, repoDir string, subPath string, filename string, content []byte) string {
	t.Helper()

	gitCmd := getGitCmd(t, repoDir)

	targetDir := repoDir
	if subPath != "" {
		targetDir = filepath.Join(targetDir, subPath)
		fi, err := os.Stat(targetDir)
		if os.IsNotExist(err) {
			if err := os.MkdirAll(targetDir, 0o700); err != nil {
				t.Fatalf("couldn't create directory %s in worktree: %v", targetDir, err)
			}
		} else if err != nil {
			t.Fatalf("checking if directory %s in worktree exists: %v", targetDir, err)
		}
		if fi != nil && !fi.IsDir() {
			t.Fatalf("%s already exists but is not a directory", targetDir)
		}
	}

	outfile := filepath.Join(targetDir, filename)
	if err := os.WriteFile(outfile, content, 0o600); err != nil {
		t.Fatalf("couldn't write content to file %s: %v", outfile, err)
	}

	if out, err := gitCmd("add", outfile).Output(); err != nil {
		t.Fatalf("couldn't add file %s to git: %q: %v", outfile, out, err)
	}

	if out, err := gitCmd("commit", "-m", "adding file for test").Output(); err != nil {
		t.Fatalf("couldn't perform commit for test: %q: %v", out, err)
	}

	out, err := gitCmd("rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("couldn't parse HEAD revision: %v", err)
	}

	return strings.TrimSpace(string(out))
}
