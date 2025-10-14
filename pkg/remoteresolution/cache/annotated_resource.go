/*
Copyright 2024 The Tekton Authors

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

package cache

import (
	"time"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

const (
	// cacheAnnotationKey is the annotation key indicating if a resource was cached
	cacheAnnotationKey = "resolution.tekton.dev/cached"
	// cacheTimestampKey is the annotation key for when the resource was cached
	cacheTimestampKey = "resolution.tekton.dev/cache-timestamp"
	// cacheResolverTypeKey is the annotation key for the resolver type that cached it
	cacheResolverTypeKey = "resolution.tekton.dev/cache-resolver-type"
	// cacheOperationKey is the annotation key for the cache operation type
	cacheOperationKey = "resolution.tekton.dev/cache-operation"
	// cacheValueTrue is the value used for cache annotations
	cacheValueTrue = "true"
	// CacheOperationStore is the value for cache store operations
	CacheOperationStore = "store"
	// CacheOperationRetrieve is the value for cache retrieve operations
	CacheOperationRetrieve = "retrieve"
)

// annotatedResource wraps a ResolvedResource with cache annotations
type annotatedResource struct {
	resource    resolutionframework.ResolvedResource
	annotations map[string]string
}

// newAnnotatedResource creates a new annotatedResource with cache annotations
func newAnnotatedResource(resource resolutionframework.ResolvedResource, resolverType, operation string) *annotatedResource {
	annotations := resource.Annotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[cacheAnnotationKey] = cacheValueTrue
	annotations[cacheTimestampKey] = time.Now().Format(time.RFC3339)
	annotations[cacheResolverTypeKey] = resolverType
	annotations[cacheOperationKey] = operation

	return &annotatedResource{
		resource:    resource,
		annotations: annotations,
	}
}

// Data returns the bytes of the resource
func (a *annotatedResource) Data() []byte {
	return a.resource.Data()
}

// Annotations returns the annotations with cache metadata
func (a *annotatedResource) Annotations() map[string]string {
	return a.annotations
}

// RefSource returns the source reference of the remote data
func (a *annotatedResource) RefSource() *v1.RefSource {
	return a.resource.RefSource()
}
