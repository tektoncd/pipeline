/*
Copyright 2022 The Tekton Authors
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

package bundle

import "github.com/tektoncd/pipeline/pkg/apis/resolution"

const (
	// BundleAnnotationKind is the image layer annotation used to indicate
	// the "kind" of resource stored in a given layer.
	BundleAnnotationKind = "dev.tekton.image.kind"

	// BundleAnnotationName is the image layer annotation used to indicate
	// the "name" of resource stored in a given layer.
	BundleAnnotationName = "dev.tekton.image.name"

	// BundleAnnotationAPIVersion is the image layer annotation used to
	// indicate the "apiVersion" of resource stored in a given layer.
	BundleAnnotationAPIVersion = "dev.tekton.image.apiVersion"
)

var (
	// ResolverAnnotationKind is the resolver annotation used to indicate
	// the "kind" of resource.
	ResolverAnnotationKind = resolution.GroupName + "/" + BundleAnnotationKind

	// ResolverAnnotationName is resolver annotation used to indicate
	// the "name" of resource.
	ResolverAnnotationName = resolution.GroupName + "/" + BundleAnnotationName

	// ResolverAnnotationAPIVersion is the resolver annotation used to
	// indicate the "apiVersion" of resource.
	ResolverAnnotationAPIVersion = resolution.GroupName + "/" + BundleAnnotationAPIVersion
)
