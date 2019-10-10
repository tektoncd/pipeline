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

package clustermanager

import (
	"fmt"

	"knative.dev/pkg/testutils/common"
)

var (
	ClusterResource ResourceType = "e2e-cls"
)

type ResourceType string

// getResourceName defines how a resource should be named based on it's
// type, the name follows: k{reponame}-{typename} for local user, and
// append BUILD_NUMBER with up to 20 chars. This is best effort, as it
// shouldn't fail
func getResourceName(rt ResourceType) (string, error) {
	var resName string
	repoName, err := common.GetRepoName()
	if err != nil {
		return "", fmt.Errorf("failed getting reponame for forming resource name: '%v'", err)
	}
	resName = fmt.Sprintf("k%s-%s", repoName, string(rt))
	if common.IsProw() {
		buildNumStr := common.GetOSEnv("BUILD_NUMBER")
		if buildNumStr == "" {
			return "", fmt.Errorf("failed getting BUILD_NUMBER env var")
		}
		if len(buildNumStr) > 20 {
			buildNumStr = string(buildNumStr[:20])
		}
		resName = fmt.Sprintf("%s-%s", resName, buildNumStr)
	}
	return resName, nil
}
