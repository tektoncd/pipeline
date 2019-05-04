// +build e2e

/*
Copyright 2019 Knative Authors LLC
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

package test

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/xerrors"
)

var (
	// Wether missing KO_DOCKER_REPO environment variable should be fatal or not
	missingKoFatal = "true"
)

func ensureDockerRepo(t *testing.T) string {
	repo, err := getDockerRepo()
	if err != nil {
		if missingKoFatal == "false" {
			t.Skip("KO_DOCKER_REPO env variable is required")
		}
		t.Fatal("KO_DOCKER_REPO env variable is required")
	}
	return repo
}

func getDockerRepo() (string, error) {
	// according to knative/test-infra readme (https://github.com/knative/test-infra/blob/13055d769cc5e1756e605fcb3bcc1c25376699f1/scripts/README.md)
	// the KO_DOCKER_REPO will be set with according to the project where the cluster is created
	// it is used here to dynamically get the docker registry to push the image to
	dockerRepo := os.Getenv("KO_DOCKER_REPO")
	if dockerRepo == "" {
		return "", xerrors.New("KO_DOCKER_REPO env variable is required")
	}
	return fmt.Sprintf("%s/kanikotasktest", dockerRepo), nil
}
