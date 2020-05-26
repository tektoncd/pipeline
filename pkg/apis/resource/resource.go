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

package resource

import (
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/cloudevent"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/cluster"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/git"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/image"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/pullrequest"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/storage"
)

// FromType returns an instance of the correct PipelineResource object type which can be
// used to add input and output containers as well as volumes to a TaskRun's pod in order to realize
// a PipelineResource in a pod.
func FromType(name string, r *resourcev1alpha1.PipelineResource, images pipeline.Images) (pipelinev1beta1.PipelineResourceInterface, error) {
	switch r.Spec.Type {
	case resourcev1alpha1.PipelineResourceTypeGit:
		return git.NewResource(name, images.GitImage, r)
	case resourcev1alpha1.PipelineResourceTypeImage:
		return image.NewResource(name, r)
	case resourcev1alpha1.PipelineResourceTypeCluster:
		return cluster.NewResource(name, images.KubeconfigWriterImage, images.ShellImage, r)
	case resourcev1alpha1.PipelineResourceTypeStorage:
		return storage.NewResource(name, images, r)
	case resourcev1alpha1.PipelineResourceTypePullRequest:
		return pullrequest.NewResource(name, images.PRImage, r)
	case resourcev1alpha1.PipelineResourceTypeCloudEvent:
		return cloudevent.NewResource(name, r)
	}
	return nil, fmt.Errorf("%s is an invalid or unimplemented PipelineResource", r.Spec.Type)
}
