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

package system

import (
	"cmp"
	"fmt"
	"os"
)

const (
	// NamespaceEnvKey is the environment variable that specifies the system namespace.
	NamespaceEnvKey = "SYSTEM_NAMESPACE"

	// ResourceLabelEnvKey is the environment variable that specifies the system resource
	// label. This label should be used to limit the number of configmaps that are watched
	// in the system namespace.
	ResourceLabelEnvKey = "SYSTEM_RESOURCE_LABEL"
)

// Namespace returns the name of the K8s namespace where our system components
// run.
func Namespace() string {
	if ns := os.Getenv(NamespaceEnvKey); ns != "" {
		return ns
	}

	panic(fmt.Sprintf(`The environment variable %q is not set

If this is a process running on Kubernetes, then it should be using the downward
API to initialize this variable via:

  env:
  - name: %s
    valueFrom:
      fieldRef:
        fieldPath: metadata.namespace

If this is a Go unit test consuming system.Namespace() then it should add the
following import:

import (
	_ "knative.dev/pkg/system/testing"
)`, NamespaceEnvKey, NamespaceEnvKey))
}

// ResourceLabel returns the label key identifying K8s objects our system
// components source their configuration from.
func ResourceLabel() string {
	return os.Getenv(ResourceLabelEnvKey)
}

// PodName will read various env vars to determine the name of the running
// pod before falling back
//
// First it will read 'POD_NAME' this is expected to be populated using the
// Kubernetes downward API.
//
//	env:
//	- name: MY_POD_NAME
//	  valueFrom:
//	    fieldRef:
//	      fieldPath: metadata.name
//
// As a fallback it will read HOSTNAME. This is undocumented
// Kubernetes behaviour that podman, cri-o and containerd have
// inherited from docker.
//
// If none of these env-vars is set PodName will return an
// empty string
func PodName() string {
	return cmp.Or(
		os.Getenv("POD_NAME"),
		os.Getenv("HOSTNAME"),
	)
}
