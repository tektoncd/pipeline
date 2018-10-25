/*
Copyright 2018 The Knative Authors.

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
	"fmt"

	v1alpha1 "github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	listers "github.com/knative/build-pipeline/pkg/client/listers/pipeline/v1alpha1"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	"go.uber.org/zap"
)

func getBoundResource(resourceName string, boundResources []v1alpha1.TaskRunResourceVersion) (*v1alpha1.TaskRunResourceVersion, error) {
	for _, br := range boundResources {
		if br.Key == resourceName {
			return &br, nil
		}
	}
	return nil, fmt.Errorf("couldnt find key %q in bound resources %s", resourceName, boundResources)
}

// AddInputResource will update the input build with the input resource from the task
func AddInputResource(
	build *buildv1alpha1.Build,
	task *v1alpha1.Task,
	taskRun *v1alpha1.TaskRun,
	pipelineResourceLister listers.PipelineResourceLister,
	logger *zap.SugaredLogger,
) (*buildv1alpha1.Build, error) {

	if task.Spec.Inputs == nil {
		return build, nil
	}

	var gitResource *v1alpha1.GitResource
	for _, input := range task.Spec.Inputs.Resources {
		boundResource, err := getBoundResource(input.Name, taskRun.Spec.Inputs.Resources)
		if err != nil {
			return nil, fmt.Errorf("Failed to get bound resource: %s", err)
		}

		resource, err := pipelineResourceLister.PipelineResources(task.Namespace).Get(boundResource.ResourceRef.Name)
		if err != nil {
			return nil, fmt.Errorf("task %q failed to Get Pipeline Resource: %q", task.Name, boundResource)
		}

		if resource.Spec.Type == v1alpha1.PipelineResourceTypeGit {
			gitResource, err = v1alpha1.NewGitResource(resource)
			if err != nil {
				return nil, fmt.Errorf("task %q invalid Pipeline Resource: %q", task.Name, boundResource.ResourceRef.Name)
			}
			if boundResource.Version != "" {
				gitResource.Revision = boundResource.Version
			}
			// TODO(#123) support mulitple git inputs
			break
		}
	}

	gitSourceSpec := &buildv1alpha1.GitSourceSpec{
		Url:      gitResource.URL,
		Revision: gitResource.Revision,
	}

	build.Spec.Source = &buildv1alpha1.SourceSpec{Git: gitSourceSpec}

	return build, nil
}
