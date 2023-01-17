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

package resources

import (
	"path/filepath"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
)

// GetOutputSteps will add the correct `path` to the output resources for pt
func GetOutputSteps(outputs map[string]*resourcev1alpha1.PipelineResource, taskName, storageBasePath string) []v1beta1.TaskResourceBinding {
	var taskOutputResources []v1beta1.TaskResourceBinding

	for name, outputResource := range outputs {
		taskOutputResource := v1beta1.TaskResourceBinding{
			PipelineResourceBinding: v1beta1.PipelineResourceBinding{
				Name: name,
			},
			Paths: []string{filepath.Join(storageBasePath, taskName, name)},
		}
		// SelfLink is being checked there to determine if this PipelineResource is an instance that
		// exists in the cluster (in which case Kubernetes will populate this field) or is specified by Spec
		if outputResource.SelfLink != "" {
			taskOutputResource.ResourceRef = &v1beta1.PipelineResourceRef{
				Name:       outputResource.Name,
				APIVersion: outputResource.APIVersion,
			}
		} else if outputResource.Spec.Type != "" {
			taskOutputResource.ResourceSpec = &resourcev1alpha1.PipelineResourceSpec{
				Type:         outputResource.Spec.Type,
				Params:       outputResource.Spec.Params,
				SecretParams: outputResource.Spec.SecretParams,
			}
		}
		taskOutputResources = append(taskOutputResources, taskOutputResource)
	}
	return taskOutputResources
}
