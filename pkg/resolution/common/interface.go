/*
Copyright 2023 The Tekton Authors

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
	"context"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResolverName is the type used for a resolver's name and is mostly
// used to ensure the function signatures that accept it are clear on the
// purpose for the given string.
type ResolverName string

// Requester is the interface implemented by a type that knows how to
// submit requests for remote resources.
type Requester interface {
	// Submit accepts the name of a resolver to submit a request to
	// along with the request itself.
	Submit(context.Context, ResolverName, Request) (ResolvedResource, error)
}

// Request is implemented by any type that represents a single request
// for a remote resource. Implementing this interface gives the underlying
// type an opportunity to control properties such as whether the name of
// a request has particular properties, whether the request should be made
// to a specific namespace, and precisely which parameters should be included.
type Request interface {
	Name() string
	Namespace() string
	Params() pipelinev1.Params
}

// OwnedRequest is implemented by any type implementing Request that also needs
// to express a Kubernetes OwnerRef relationship as part of the request being
// made.
type OwnedRequest interface {
	OwnerRef() metav1.OwnerReference
}

// ResolvedResource is implemented by any type that offers a read-only
// view of the data and metadata of a resolved remote resource.
type ResolvedResource interface {
	Data() ([]byte, error)
	Annotations() map[string]string
	RefSource() *pipelinev1.RefSource
}
