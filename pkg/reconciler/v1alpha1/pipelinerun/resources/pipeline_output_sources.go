/*
Copyright 2018 The Knative Authors

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

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var (
	pvcDir   = "/pvc"
	pvcMount = corev1.VolumeMount{
		Name:      "test", // how to choose this path
		MountPath: pvcDir, // nothing should be mounted here
	}
)

func GetOutputSteps(outputs []v1alpha1.SourceBinding, taskName string) ([]v1alpha1.TaskRunResourceVersion, []v1alpha1.TaskBuildStep) {
	var taskOutputResources []v1alpha1.TaskRunResourceVersion
	var postSteps []v1alpha1.TaskBuildStep
	for _, osb := range outputs {
		taskOutputResources = append(taskOutputResources, v1alpha1.TaskRunResourceVersion{
			ResourceRef: osb.ResourceRef,
			Name:        osb.Name,
		})

		postSteps = append(postSteps,
			v1alpha1.TaskBuildStep{
				Name:  osb.ResourceRef.Name,
				Paths: []string{filepath.Join(pvcDir, taskName, osb.Name)},
			})
	}

	return taskOutputResources, postSteps
}

func GetInputSteps(inputs []v1alpha1.SourceBinding, taskName string) ([]v1alpha1.TaskRunResourceVersion, []v1alpha1.TaskBuildStep) {
	var taskInputResources []v1alpha1.TaskRunResourceVersion
	var preSteps []v1alpha1.TaskBuildStep

	for _, isb := range inputs {
		taskInputResources = append(taskInputResources, v1alpha1.TaskRunResourceVersion{
			ResourceRef: isb.ResourceRef,
			Name:        isb.Name,
		})

		var stepSourceNames []string
		for _, constr := range isb.PassedConstraints {
			stepSourceNames = append(stepSourceNames, filepath.Join(pvcDir, constr, isb.Name))
		}
		if len(stepSourceNames) > 0 {
			preSteps = append(preSteps,
				v1alpha1.TaskBuildStep{
					Name:  isb.ResourceRef.Name,
					Paths: stepSourceNames,
				},
			)
		}
	}
	return taskInputResources, preSteps
}
