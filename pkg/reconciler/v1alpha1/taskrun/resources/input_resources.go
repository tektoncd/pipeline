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
	v1alpha1 "github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	listers "github.com/knative/build-pipeline/pkg/client/listers/pipeline/v1alpha1"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	"go.uber.org/zap"
)

// AddInputResource will update the input build with the input resource from the task
func AddInputResource(
	build *buildv1alpha1.Build,
	task *v1alpha1.Task,
	taskRun *v1alpha1.TaskRun,
	pipelineResourceLister listers.PipelineResourceLister,
	logger *zap.SugaredLogger,
) (*buildv1alpha1.Build, error) {

	var gitResource *v1alpha1.GitResource

	for _, input := range task.Spec.Inputs.Resources {
		resource, err := pipelineResourceLister.PipelineResources(task.Namespace).Get(input.Name)
		if err != nil {
			logger.Errorf("Failed to reconcile TaskRun: %q failed to Get Pipeline Resource: %q", task.Name, input.Name)
			return nil, err
		}
		if resource.Spec.Type == v1alpha1.PipelineResourceTypeGit {
			gitResource, err = v1alpha1.NewGitResource(resource)
			if err != nil {
				logger.Errorf("Failed to reconcile TaskRun: %q Invalid Pipeline Resource: %q", task.Name, input.Name)
				return nil, err
			}
			for _, trInput := range taskRun.Spec.Inputs.Resources {
				if trInput.ResourceRef.Name == input.Name {
					if trInput.Version != "" {
						gitResource.Revision = trInput.Version
					}
					break
				}
			}
			break
		}
	}

	gitSourceSpec := &buildv1alpha1.GitSourceSpec{
		Url:      gitResource.URL,
		Revision: gitResource.Revision,
	}

	build.Spec.Source = &buildv1alpha1.SourceSpec{Git: gitSourceSpec}
	// add service account name if available, otherwise Build will
	// use the default service account in the namespace
	if gitResource.ServiceAccount != "" {
		build.Spec.ServiceAccountName = gitResource.ServiceAccount
	}

	return build, nil
}
