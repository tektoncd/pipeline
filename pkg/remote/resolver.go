/*
Copyright 2019 The Tekton Authors

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

package remote

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// Resolver will retrieve Tekton resources like Tasks from remote repositories like an OCI image repositories.
type Resolver interface {
	GetTask(taskName string) (*v1beta1.TaskSpec, error)
}

// TODO: Right now, there is only one resolver type. When more are added, this will need to be updated.
func NewResolver(imageReference string, serviceAccountName string) Resolver {
	return OCIResolver{
		imageReference: imageReference,
		keychainProvider: func() (authn.Keychain, error) {
			return k8schain.NewInCluster(k8schain.Options{ServiceAccountName: serviceAccountName})
		},
	}
}
