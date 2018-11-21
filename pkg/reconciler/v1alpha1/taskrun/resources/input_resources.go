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
	"flag"
	"fmt"
	"strings"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	listers "github.com/knative/build-pipeline/pkg/client/listers/pipeline/v1alpha1"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

var kubeconfigWriterImage = flag.String("kubeconfig-writer-image", "override-with-kubeconfig-writer:latest", "The container image containing our kubeconfig writer binary.")

func getBoundResource(resourceName string, boundResources []v1alpha1.TaskRunResource) (*v1alpha1.TaskRunResource, error) {
	for _, br := range boundResources {
		if br.Name == resourceName {
			return &br, nil
		}
	}
	return nil, fmt.Errorf("couldnt find resource named %q in bound resources %s", resourceName, boundResources)
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

		switch resource.Spec.Type {
		case v1alpha1.PipelineResourceTypeGit:
			{
				gitResource, err = v1alpha1.NewGitResource(resource)
				if err != nil {
					return nil, fmt.Errorf("task %q invalid Pipeline Resource: %q", task.Name, boundResource.ResourceRef.Name)
				}
				gitSourceSpec := &buildv1alpha1.GitSourceSpec{
					Url:      gitResource.URL,
					Revision: gitResource.Revision,
				}
				build.Spec.Source = &buildv1alpha1.SourceSpec{Git: gitSourceSpec}
			}
		case v1alpha1.PipelineResourceTypeCluster:
			clusterResource, err := v1alpha1.NewClusterResource(resource)
			if err != nil {
				return nil, fmt.Errorf("task %q invalid Pipeline Resource: %q", task.Name, boundResource.ResourceRef.Name)
			}
			addClusterBuildStep(build, clusterResource)
		}
	}

	return build, nil
}

func addClusterBuildStep(build *buildv1alpha1.Build, clusterResource *v1alpha1.ClusterResource) {
	var envVars []corev1.EnvVar
	for _, sec := range clusterResource.Secrets {
		ev := corev1.EnvVar{
			Name: strings.ToUpper(sec.FieldName),
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: sec.SecretName,
					},
					Key: sec.SecretKey,
				},
			},
		}
		envVars = append(envVars, ev)
	}

	clusterContainer := corev1.Container{
		Name:  "kubeconfig",
		Image: *kubeconfigWriterImage,
		Args: []string{
			"-clusterConfig", clusterResource.String(),
		},
		Env: envVars,
	}

	buildSteps := append([]corev1.Container{clusterContainer}, build.Spec.Steps...)
	build.Spec.Steps = buildSteps
}
