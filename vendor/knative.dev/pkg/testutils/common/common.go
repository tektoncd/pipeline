/*
Copyright 2019 The Knative Authors

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

package common

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
)

var (
	// GetOSEnv is for easier mocking in unit tests
	GetOSEnv = os.Getenv
	// StandardExec is for easier mocking in unit tests
	StandardExec = standardExec
)

// standardExec executes shell command and returns stdout and stderr
func standardExec(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

// IsProw checks if the process is initialized by Prow
func IsProw() bool {
	return GetOSEnv("PROW_JOB_ID") != ""
}

// GetRepoName gets repo name by the path where the repo cloned to
func GetRepoName() (string, error) {
	out, err := StandardExec("git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("failed git rev-parse --show-toplevel: '%v'", err)
	}
	return strings.TrimSpace(path.Base(string(out))), nil
}
