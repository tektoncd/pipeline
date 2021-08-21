// +build e2e

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

// Package pullrequest tests the pullrequest-init binary end-to-end by invoking
// the binary and observing the resulting filesystem state.
package pullrequest

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/go-scm/scm"
)

const (
	pkg = "github.com/tektoncd/pipeline/cmd/pullrequest-init"
)

var (
	proxy = flag.Bool("use-fake-scm-proxy", true, "whether to use fake in-memory proxy instead of using real SCM servers")
)

func TestPullRequest(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", t.Name())
	if err != nil {
		t.Fatalf("could not create test temp dir: %v", err)
	}
	t.Logf("Test dir: %s", tmpdir)
	defer os.RemoveAll(tmpdir)

	updateDeps(t)

	for _, tc := range []struct {
		name string
		url  string

		// Location of fake SCM data to use for proxy.
		scmData string

		// Auth token to use.
		token string
		// If auth token present, fmt template for Authorization header to check.
		authHeader string
	}{
		{
			name: "github-default",
			// Must be http so we can intercept the request in the proxy.
			url:        "http://github.com/wlynch/test/pulls/1",
			scmData:    filepath.Join("testdata", "scm", "api.github.com"),
			token:      envToken("GITHUB_TOKEN"),
			authHeader: "Bearer %s",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(tmpdir, tc.name)
			if err := os.Mkdir(dir, 0755); err != nil {
				t.Fatalf("os.Mkdir: %v", err)
			}

			// If provided, set the AUTH_TOKEN environment variable that the
			// pullrequest binary expects.
			if tc.token != "" {
				os.Setenv("AUTH_TOKEN", tc.token)
				defer os.Unsetenv("AUTH_TOKEN")
			}

			if *proxy {
				var header http.Header
				if tc.token != "" {
					header = http.Header{
						"Authorization": []string{fmt.Sprintf(tc.authHeader, tc.token)},
					}
				}
				startProxy(t, tc.scmData, header)
			}

			t.Log("Environment vars:", os.Environ())

			// Download PR data to local dir.
			runPRBinary(t, "download", dir, tc.url)
			// Diff downloaded contents with golden copies.
			golden := filepath.Join("testdata", "golden")
			if diff, err := exec.Command("diff", "-ruw", golden, dir).CombinedOutput(); err != nil {
				t.Error(string(diff))
			}

			// Modify PR resource contents.
			modifyFiles(t, dir)

			// Upload new state back to server.
			runPRBinary(t, "upload", dir, tc.url)
		})
	}
}

// Stand up fake proxy server. This should not be called if you want to talk to
// real SCM providers, since this will intercept all HTTP traffic.
func startProxy(t *testing.T, dataPath string, header http.Header) {
	t.Helper()

	h := &handler{
		Data:   dataPath,
		Header: header,
	}

	// Start server on any open port.
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	t.Logf("starting HTTP proxy on %s", l.Addr().String())
	go func() {
		if err := http.Serve(l, h); err != nil {
			fmt.Println("http.Serve:", err)
		}
	}()
	os.Setenv("http_proxy", l.Addr().String())
}

// updateDeps makes sure command is installed locally to resolve any
// dependencies. We need to do this first since the run might try to
// pull in new dependencies through the proxy.
func updateDeps(t *testing.T) {
	t.Helper()
	install := exec.Command("go", "build", pkg)
	t.Log(install)
	if out, err := install.CombinedOutput(); err != nil {
		t.Fatalf("failed to update dependencies: %s", string(out))
	}
}

// runPRBinary invokes the package binary. If the command returns an error
// the test fails and stdout/stderr are included in the error logs.
func runPRBinary(t *testing.T, mode, path, url string) {
	t.Helper()
	banner(t, mode)
	flags := []string{
		"run",
		pkg,
		fmt.Sprintf("-mode=%s", mode),
		fmt.Sprintf("-path=%s", path),
		fmt.Sprintf("-url=%s", url),
	}
	cmd := exec.Command("go", flags...)
	t.Log(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run %s: %s", mode, string(out))
	}
}

// modifyFiles edits a set of files in the target PR directory to make
// a user change in the pull request.
func modifyFiles(t *testing.T, dir string) {
	t.Helper()
	banner(t, "modify")
	for path, v := range map[string]interface{}{
		filepath.Join(dir, "status", "new.json"): &scm.Status{
			State:  scm.StateSuccess,
			Label:  "tekton-e2e",
			Desc:   "tekton-e2e",
			Target: "https://tekton.dev",
		},
	} {
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("os.Create(%s): %v", path, err)
		}
		defer f.Close()
		if err := json.NewEncoder(f).Encode(v); err != nil {
			t.Fatalf("json.Encode(%s): %v", path, err)
		}
	}
}

func banner(t *testing.T, s string) {
	t.Helper()
	t.Logf("\n########################################\n# %s\n########################################", s)
}

// envToken looks for a test specific environment variable (e.g. GITHUB_TOKEN).
// If none is set, it falls back to the default "AUTH_TOKEN" environment
// variable.
// If AUTH_TOKEN is not set, it falls back to a static token. This token is not
// valid for real environments, but will work for the local proxy.
func envToken(env string) string {
	t := os.Getenv(env)
	if t == "" {
		t = os.Getenv("AUTH_TOKEN")
	}
	if t == "" {
		return "hunter2"
	}
	return t
}
