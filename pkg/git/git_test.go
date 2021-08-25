/*
Copyright 2020 The Tekton Authors

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
	"bufio"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestValidateGitSSHURLFormat(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{
			url:  "git@github.com:user/project.git",
			want: true,
		},
		{
			url:  "git@127.0.0.1:user/project.git",
			want: true,
		},
		{
			url:  "http://github.com/user/project.git",
			want: false,
		},
		{
			url:  "https://github.com/user/project.git",
			want: false,
		},
		{
			url:  "http://127.0.0.1/user/project.git",
			want: false,
		},
		{
			url:  "https://127.0.0.1/user/project.git",
			want: false,
		},
		{
			url:  "http://host.xz/path/to/repo.git/",
			want: false,
		},
		{
			url:  "https://host.xz/path/to/repo.git/",
			want: false,
		},
		{
			url:  "ssh://user@host.xz:port/path/to/repo.git/",
			want: true,
		},
		{
			url:  "ssh://user@host.xz/path/to/repo.git/",
			want: true,
		},
		{
			url:  "ssh://host.xz:port/path/to/repo.git/",
			want: true,
		},
		{
			url:  "ssh://host.xz/path/to/repo.git/",
			want: true,
		},
		{
			url:  "git://host.xz/path/to/repo.git/",
			want: false,
		},
		{
			url:  "/path/to/repo.git/",
			want: false,
		},
		{
			url:  "file://~/path/to/repo.git/",
			want: false,
		},
		{
			url:  "user@host.xz:/path/to/repo.git/",
			want: true,
		},
		{
			url:  "host.xz:/path/to/repo.git/",
			want: true,
		},
		{
			url:  "user@host.xz:path/to/repo.git",
			want: true,
		},
	}

	for _, tt := range tests {
		got := validateGitSSHURLFormat(tt.url)
		if got != tt.want {
			t.Errorf("Validate URL(%v)'s SSH format got %v, want %v", tt.url, got, tt.want)
		}
	}
}

func Test_validateGitAuth(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		logMessage string
		wantSSHdir bool
	}{
		{
			name:       "Validate HTTP Auth",
			url:        "http://google.com",
			logMessage: "",
			wantSSHdir: false,
		},
		{
			name:       "SSH URL but no SSH credentials",
			url:        "ssh://git@github.com:chmouel/tekton",
			logMessage: "URL(\"ssh://git@github.com:chmouel/tekton\") appears to need SSH authentication but no SSH credentials have been provided",
			wantSSHdir: false,
		},
		{
			name:       "Validate SSH Auth",
			url:        "http://github.com/chmouel/tekton",
			logMessage: "SSH credentials have been provided but the URL(\"http://github.com/chmouel/tekton\") is not a valid SSH URL. This warning can be safely ignored if the URL is for a public repo or you are using basic auth",
			wantSSHdir: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observer, log := observer.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			credsDir, cleanup := createTempDir(t)
			defer cleanup()
			if tt.wantSSHdir {
				os.MkdirAll(credsDir+"/.ssh", 0755)
			}

			validateGitAuth(logger, credsDir, tt.url)
			if tt.logMessage != "" {
				takeAll := log.TakeAll()
				if len(takeAll) == 0 {
					t.Fatal("We didn't receive any logging")
				}
				gotmsg := takeAll[0].Message
				if tt.logMessage != gotmsg {
					t.Errorf("log message: '%s'\n should be '%s'", tt.logMessage, gotmsg)
				}
			}
		})
	}
}

func TestFetch(t *testing.T) {
	tests := []struct {
		name       string
		logMessage string
		spec       FetchSpec
		wantErr    bool
	}{
		{
			name:       "test-good",
			logMessage: "Successfully cloned",
			wantErr:    false,
			spec: FetchSpec{
				URL:                       "",
				Revision:                  "",
				Refspec:                   "",
				Path:                      "",
				Depth:                     0,
				Submodules:                false,
				SSLVerify:                 false,
				HTTPProxy:                 "",
				HTTPSProxy:                "",
				NOProxy:                   "",
				SparseCheckoutDirectories: "",
			},
		}, {
			name:       "test-clone-with-sparse-checkout",
			logMessage: "Successfully cloned",
			wantErr:    false,
			spec: FetchSpec{
				URL:                       "",
				Revision:                  "",
				Refspec:                   "",
				Path:                      "",
				Depth:                     0,
				Submodules:                false,
				SSLVerify:                 false,
				HTTPProxy:                 "",
				HTTPSProxy:                "",
				NOProxy:                   "",
				SparseCheckoutDirectories: "a,b/c",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observer, log := observer.New(zap.InfoLevel)
			defer func() {
				for _, line := range log.TakeAll() {
					t.Logf("[%q git]: %s", line.Level, line.Message)
				}
			}()
			logger := zap.New(observer).Sugar()

			gitDir, cleanup := createTempDir(t)
			defer cleanup()
			createTempGit(t, logger, gitDir)
			tt.spec.URL = gitDir

			targetPath, cleanup2 := createTempDir(t)
			defer cleanup2()
			tt.spec.Path = targetPath

			if err := Fetch(logger, tt.spec); (err != nil) != tt.wantErr {
				t.Errorf("Fetch() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.spec.SparseCheckoutDirectories != "" {
				dirPatterns := strings.Split(tt.spec.SparseCheckoutDirectories, ",")

				sparseFile, err := os.Open(".git/info/sparse-checkout")
				if err != nil {
					t.Fatal("Unable to read sparse-checkout file")
				}
				defer sparseFile.Close()

				var sparsePatterns []string

				scanner := bufio.NewScanner(sparseFile)
				for scanner.Scan() {
					sparsePatterns = append(sparsePatterns, scanner.Text())
				}

				if cmp.Diff(dirPatterns, sparsePatterns) != "" {
					t.Errorf("directory patterns and sparse-checkout patterns do not match")
				}
			}

			if tt.logMessage != "" {
				allLogLines := log.All()
				if len(allLogLines) == 0 {
					t.Fatal("We didn't receive any logging")
				}
				gotmsg := allLogLines[0].Message
				if !strings.Contains(gotmsg, tt.logMessage) {
					t.Errorf("log message: '%s'\n should contains: '%s'", tt.logMessage, gotmsg)
				}
			}
		})
	}
}

func createTempDir(t *testing.T) (string, func()) {
	dir, err := ioutil.TempDir("", "git-init-")
	if err != nil {
		t.Fatalf("unexpected error creating temp directory: %v", err)
	}
	return dir, func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("unexpected error cleaning up temp directory: %v", err)
		}
	}
}

// Create a temporary Git dir locally for testing against instead of using a potentially flaky remote URL.
func createTempGit(t *testing.T, logger *zap.SugaredLogger, gitDir string) {
	if _, err := run(logger, "", "init", gitDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(gitDir); err != nil {
		t.Fatalf("failed to change directory with path %s; err: %v", gitDir, err)
	}
	if _, err := run(logger, "", "checkout", "-b", "main"); err != nil {
		t.Fatal(err)
	}

	// Not defining globally so we don't mess with the global gitconfig
	if _, err := run(logger, "", "config", "user.email", "tester@tekton.dev"); err != nil {
		t.Fatal(err)
	}

	// Not defining globally so we don't mess with the global gitconfig
	if _, err := run(logger, "", "config", "user.name", "Tekton Test"); err != nil {
		t.Fatal(err)
	}

	if _, err := run(logger, "", "commit", "--allow-empty", "-m", "Hello Moto"); err != nil {
		t.Fatal(err.Error())
	}
}
