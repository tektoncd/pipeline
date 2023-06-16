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
	"context"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ResolvedObject is returned by Resolver.List representing a Tekton resource stored in a remote location.
type ResolvedObject struct {
	Kind       string
	APIVersion string
	Name       string
}

// Resolver defines a generic API to retrieve Tekton resources from remote locations. It allows 2 principle operations:
//   - List:     retrieve a flat set of Tekton objects in this remote location
//   - Get:      retrieves a specific object with the given Kind and name, and the refSource identifying where the resource came from.
type Resolver interface {
	List(ctx context.Context) ([]ResolvedObject, error)
	Get(ctx context.Context, kind, name string) (runtime.Object, *v1.RefSource, error)
}
