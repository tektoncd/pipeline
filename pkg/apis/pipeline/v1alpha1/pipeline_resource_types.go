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

package v1alpha1

import (
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
)

// PipelineResourceType represents the type of endpoint the pipelineResource is, so that the
// controller will know this pipelineResource should be fetched and optionally what
// additional metatdata should be provided for it.
type PipelineResourceType = resource.PipelineResourceType

const (
	// PipelineResourceTypeGit indicates that this source is a Git repo.
	PipelineResourceTypeGit PipelineResourceType = resource.PipelineResourceTypeGit

	// PipelineResourceTypeStorage indicates that this source is a storage blob resource.
	PipelineResourceTypeStorage PipelineResourceType = resource.PipelineResourceTypeStorage

	// PipelineResourceTypeImage indicates that this source is a docker Image.
	PipelineResourceTypeImage PipelineResourceType = resource.PipelineResourceTypeImage

	// PipelineResourceTypeCluster indicates that this source is a k8s cluster Image.
	PipelineResourceTypeCluster PipelineResourceType = resource.PipelineResourceTypeCluster

	// PipelineResourceTypePullRequest indicates that this source is a SCM Pull Request.
	PipelineResourceTypePullRequest PipelineResourceType = resource.PipelineResourceTypePullRequest

	// PipelineResourceTypeCloudEvent indicates that this source is a cloud event URI
	PipelineResourceTypeCloudEvent PipelineResourceType = resource.PipelineResourceTypeCloudEvent
)

// AllResourceTypes can be used for validation to check if a provided Resource type is one of the known types.
var AllResourceTypes = resource.AllResourceTypes

// PipelineResource describes a resource that is an input to or output from a
// Task.
//
type PipelineResource = resource.PipelineResource

// PipelineResourceSpec defines  an individual resources used in the pipeline.
type PipelineResourceSpec = resource.PipelineResourceSpec

// SecretParam indicates which secret can be used to populate a field of the resource
type SecretParam = resource.SecretParam

// ResourceParam declares a string value to use for the parameter called Name, and is used in
// the specific context of PipelineResources.
type ResourceParam = resource.ResourceParam

// ResourceDeclaration defines an input or output PipelineResource declared as a requirement
// by another type such as a Task or Condition. The Name field will be used to refer to these
// PipelineResources within the type's definition, and when provided as an Input, the Name will be the
// path to the volume mounted containing this PipelineResource as an input (e.g.
// an input Resource named `workspace` will be mounted at `/workspace`).
type ResourceDeclaration = resource.ResourceDeclaration

// PipelineResourceList contains a list of PipelineResources
type PipelineResourceList = resource.PipelineResourceList
