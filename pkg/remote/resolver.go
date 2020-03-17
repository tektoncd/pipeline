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
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"k8s.io/client-go/kubernetes"
)

// Resolver will retreive Tekton resources like Tasks from remote repositories like an OCI image repository.
type Resolver interface {
	GetTask(string) (v1alpha1.TaskInterface, error)
}

// ImageTaskResolver will return a GetTask function capable of returning a Task from an OCI compliant image using the
// image pull secrets of the included service account.
func ImageTaskResolver(k8s kubernetes.Interface, imgRef, ns, serviceAccountName string) (resources.GetTask, error) {
	kc, err := k8schain.New(k8s, k8schain.Options{
		Namespace:          ns,
		ServiceAccountName: serviceAccountName,
	})
	if err != nil {
		return nil, err
	}
	resolver := NewOCIResolver(imgRef, kc)
	return resolver.GetTask, nil
}
