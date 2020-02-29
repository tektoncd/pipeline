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

package version

import (
	"fmt"
	"os"
	"strings"

	"github.com/blang/semver"
	"k8s.io/client-go/discovery"
)

const (
	// KubernetesMinVersionKey is the environment variable that can be used to override
	// the Kubernetes minimum version required by Knative.
	KubernetesMinVersionKey = "KUBERNETES_MIN_VERSION"

	defaultMinimumVersion = "v1.15.1"
)

func getMinimumVersion() string {
	if v := os.Getenv(KubernetesMinVersionKey); v != "" {
		return v
	}
	return defaultMinimumVersion
}

// CheckMinimumVersion checks if the currently installed version of
// Kubernetes is compatible with the minimum version required.
// Returns an error if its not.
//
// A Kubernetes discovery client can be passed in as the versioner
// like `CheckMinimumVersion(kubeClient.Discovery())`.
func CheckMinimumVersion(versioner discovery.ServerVersionInterface) error {
	v, err := versioner.ServerVersion()
	if err != nil {
		return err
	}

	currentVersion, err := semver.Make(normalizeVersion(v.GitVersion))
	if err != nil {
		return err
	}
	minimumVersion, err := semver.Make(normalizeVersion(getMinimumVersion()))
	if err != nil {
		return err
	}

	// Compare returns 1 if the first version is greater than the
	// second version.
	if currentVersion.LT(minimumVersion) {
		return fmt.Errorf("kubernetes version %q is not compatible, need at least %q (this can be overridden with the env var %q)",
			currentVersion, minimumVersion, KubernetesMinVersionKey)
	}
	return nil
}

func normalizeVersion(v string) string {
	if strings.HasPrefix(v, "v") {
		// No need to account for unicode widths.
		return v[1:]
	}
	return v
}
