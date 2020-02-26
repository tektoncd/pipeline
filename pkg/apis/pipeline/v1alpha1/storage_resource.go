/*
Copyright 2019-2020 The Tekton Authors

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

package v1alpha1

import (
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
)

const (
	// PipelineResourceTypeGCS is the subtype for the GCSResources, which is backed by a GCS blob/directory.
	PipelineResourceTypeGCS PipelineResourceType = resource.PipelineResourceTypeGCS

	// PipelineResourceTypeBuildGCS is the subtype for the BuildGCSResources, which is simialr to the GCSResource but
	// with additional functionality that was added to be compatible with knative build.
	PipelineResourceTypeBuildGCS PipelineResourceType = resource.PipelineResourceTypeBuildGCS
)
