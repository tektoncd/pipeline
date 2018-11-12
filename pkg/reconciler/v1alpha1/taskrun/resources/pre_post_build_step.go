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

	"fmt"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var (
	pvcDir       = "/pvc"
	workspaceDir = "/workspace"
)

// AddAfterSteps adds steps to build that can run after all build steps have completed
// if source has targetPath specified then step will use that path as source dir for copy command else
// it uses /workspace as default directory
func AddAfterSteps(
	taskRunSpec v1alpha1.TaskRunSpec,
	b *buildv1alpha1.Build,
	pvcName string,
) bool {
	var postBuildSteps = make(map[string][]string)
	var needPVC bool
	for _, postStepResource := range taskRunSpec.Outputs.Resources {
		postBuildSteps[postStepResource.ResourceRef.Name] = postStepResource.Paths
	}
	for _, source := range b.Spec.Sources {
		if paths, ok := postBuildSteps[source.Name]; ok {
			var newSteps []corev1.Container
			for _, path := range paths {
				var sPath string
				if source.TargetPath == "" {
					sPath = workspaceDir
				} else {
					sPath = filepath.Join(workspaceDir, source.TargetPath)
				}
				newSteps = append(newSteps, []corev1.Container{{
					Name:         fmt.Sprintf("source-mkdir-%s", source.Name),
					Image:        "busybox",
					Command:      []string{"mkdir"},
					Args:         []string{"-p", path},
					VolumeMounts: []corev1.VolumeMount{getPvcMount(pvcName)},
				}, {
					Name:         fmt.Sprintf("source-copy-%s", source.Name),
					Image:        "busybox",
					Command:      []string{"cp"},
					Args:         []string{"-r", fmt.Sprintf("%s/.", sPath), path},
					VolumeMounts: []corev1.VolumeMount{getPvcMount(pvcName)},
				},
				}...)
			}
			if len(newSteps) > 0 {
				needPVC = true
				b.Spec.Steps = append(b.Spec.Steps, newSteps...)
			}
		}
	}
	return needPVC
}

// AddBeforeSteps adds steps to run after sources are mounted but before actual build steps to override
// the source folder with previous task's source
func AddBeforeSteps(
	taskRunSpec v1alpha1.TaskRunSpec,
	b *buildv1alpha1.Build,
	pvcName string,
) bool {
	var needPVC bool
	var preSteps = make(map[string][]string)
	for _, preStepResource := range taskRunSpec.Inputs.Resources {
		preSteps[preStepResource.ResourceRef.Name] = preStepResource.Paths
	}

	for _, source := range b.Spec.Sources {
		if paths, ok := preSteps[source.Name]; ok {
			var newSteps []corev1.Container
			for i, path := range paths {
				var dPath string
				if source.TargetPath == "" {
					dPath = workspaceDir
				} else {
					dPath = filepath.Join(workspaceDir, source.TargetPath)
				}
				newSteps = append(newSteps, corev1.Container{
					Name:         fmt.Sprintf("source-copy-%s-%d", source.Name, i),
					Image:        "busybox",
					Command:      []string{"cp"},
					Args:         []string{"-r", fmt.Sprintf("%s/.", path), dPath},
					VolumeMounts: []corev1.VolumeMount{getPvcMount(pvcName)},
				})
			}
			if len(newSteps) > 0 {
				needPVC = true
				b.Spec.Steps = append(newSteps, b.Spec.Steps...)
			}
		}
	}
	return needPVC
}

func getPvcMount(name string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      name,   // taskrun pvc name
		MountPath: pvcDir, // nothing should be mounted here
	}
}

// GetPVCVolume gets pipelinerun pvc
func GetPVCVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: name},
		},
	}
}
